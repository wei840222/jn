package main

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/ratelimit"
	v8 "rogchap.com/v8go"
)

//go:embed third_party/js/*
var jslib embed.FS

type jsLibrary struct {
	name          string
	source        string
	script        *v8.UnboundScript
	compilerCache *v8.CompilerCachedData
}

type jsHandler struct {
	v8Iso  *v8.Isolate
	jslibs []*jsLibrary
	cl     chan struct{}
	rl     ratelimit.Limiter

	jsInvokeConcurrencyMetrics syncint64.UpDownCounter
}

func (h *jsHandler) allowLimit() func() {
	h.cl <- struct{}{}
	h.rl.Take()
	return func() {
		<-h.cl
	}
}

func (h *jsHandler) invoke(c *gin.Context) {
	span := trace.SpanFromContext(c)

	done := h.allowLimit()
	defer done()
	h.jsInvokeConcurrencyMetrics.Add(c, 1)
	defer h.jsInvokeConcurrencyMetrics.Add(c, -1)
	span.AddEvent("allowLimit", trace.WithAttributes(attribute.Int("concurrency", len(h.cl))))

	var req struct {
		Script string `json:"script" binding:"required"`
		Data   any    `json:"data"`
	}
	if strings.Contains(strings.ToLower(string(c.ContentType())), "application/json") {
		if err := c.ShouldBind(&req); err != nil {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
	} else {
		req.Data, _ = readMultipartTextOrFile(c, "data")
		script, err := readMultipartTextOrFile(c, "script")
		if err != nil {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		req.Script = script
	}
	span.AddEvent("script", trace.WithAttributes(attribute.String("script", req.Script)))

	v8Ctx := v8.NewContext(h.v8Iso)
	defer v8Ctx.Close()

	_, sspan := tracer.Start(c, "try set data to v8 global variable")
	if req.Data != nil {
		global := v8Ctx.Global()
		if s, ok := req.Data.(string); ok {
			span.AddEvent("data", trace.WithAttributes(attribute.String("data", s)))
			if err := global.Set("data", s); err != nil {
				panic(err)
			}
		} else {
			b, err := json.Marshal(req.Data)
			if err != nil {
				panic(err)
			}
			span.AddEvent("data", trace.WithAttributes(attribute.String("data", string(b))))
			if err := global.Set("data", string(b)); err != nil {
				panic(err)
			}
		}
	}

	if _, err := v8Ctx.RunScript("try { data = JSON.parse(data) } catch {}", "parse.js"); err != nil {
		panic(err)
	}
	sspan.End()

	_, sspan = tracer.Start(c, "load javascript library")
	for _, jslib := range h.jslibs {
		if _, err := jslib.script.Run(v8Ctx); err != nil {
			panic(err)
		}
		sspan.AddEvent("load " + jslib.name)
	}
	sspan.End()

	_, sspan = tracer.Start(c, "run javascript")
	result, err := v8Ctx.RunScript(req.Script, "script.js")
	if err != nil {
		if jsErr, ok := err.(*v8.JSError); ok {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
				"error":      jsErr.Message,
				"source":     jsErr.Location,
				"stackTrace": jsErr.StackTrace,
			})
			sspan.RecordError(err, trace.WithAttributes(
				attribute.String("error", jsErr.Message),
				attribute.String("source", jsErr.Location),
				attribute.String("stackTrace", jsErr.StackTrace),
			))
			return
		}
		panic(err)
	}
	sspan.End()

	if result.IsNullOrUndefined() {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "The output of script is null or undefined, please make sure that the last line of your script contains the variables to be output.",
		})
		return
	}

	if result.IsObject() {
		c.JSON(http.StatusOK, gin.H{
			"result": result.Object(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": result.String(),
	})
}

func RegisterJSHandler(lc fx.Lifecycle, e *gin.Engine) error {
	jsInvokeConcurrencyUpDownCounter, err := meter.SyncInt64().UpDownCounter("js_invoke_concurrency", instrument.WithDescription("Current concurrency of JavaScript invocation."))
	if err != nil {
		return err
	}

	h := &jsHandler{
		v8Iso:  v8.NewIsolate(),
		jslibs: make([]*jsLibrary, 0),
		cl:     make(chan struct{}, 10),
		rl:     ratelimit.New(1000),

		jsInvokeConcurrencyMetrics: jsInvokeConcurrencyUpDownCounter,
	}

	e.POST("/invoke/js", h.invoke)

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			v8Ctx := v8.NewContext(h.v8Iso)
			defer v8Ctx.Close()
			return fs.WalkDir(jslib, ".", func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				if strings.HasSuffix(path, ".js") {
					b, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					info, err := entry.Info()
					if err != nil {
						return err
					}
					script, err := h.v8Iso.CompileUnboundScript(string(b), info.Name(), v8.CompileOptions{})
					if err != nil {
						return err
					}
					script.Run(v8Ctx)
					h.jslibs = append(h.jslibs, &jsLibrary{
						name:          info.Name(),
						source:        string(b),
						script:        script,
						compilerCache: script.CreateCodeCache(),
					})
				}
				return nil
			})
		},
		OnStop: func(context.Context) error {
			h.v8Iso.Dispose()
			return nil
		},
	})

	return nil
}
