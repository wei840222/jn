package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"rogchap.com/v8go"
)

//go:embed third_party/js/*
var jslib embed.FS

type jsLibrary struct {
	name          string
	source        string
	compilerCache *v8go.CompilerCachedData
}

type jsHandler struct {
	jslibs   []*jsLibrary
	v8goIsos chan *v8go.Isolate

	jsInvokeConcurrencyMetrics metric.Int64UpDownCounter
}

func (h *jsHandler) loadJSLibrary() error {
	h.jslibs = nil

	v8goIso := v8go.NewIsolate()
	defer v8goIso.Dispose()
	v8goCtx := v8go.NewContext(v8goIso)
	defer v8goCtx.Close()

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
			script, err := v8goIso.CompileUnboundScript(string(b), info.Name(), v8go.CompileOptions{})
			if err != nil {
				return err
			}
			script.Run(v8goCtx)
			h.jslibs = append(h.jslibs, &jsLibrary{
				name:          info.Name(),
				source:        string(b),
				compilerCache: script.CreateCodeCache(),
			})
		}
		return nil
	})
}

func (h *jsHandler) generateV8GoContext(ctx context.Context) (*v8go.Context, func()) {
	_, span := tracer.Start(ctx, "load javascript library")
	defer span.End()

	v8goIso := <-h.v8goIsos
	v8goCtx := v8go.NewContext(v8goIso)

	for _, jslib := range h.jslibs {
		script, err := v8goIso.CompileUnboundScript(jslib.source, jslib.name, v8go.CompileOptions{CachedData: jslib.compilerCache})
		if err != nil {
			panic(err)
		}
		span.AddEvent("compile " + jslib.name)
		if _, err := script.Run(v8goCtx); err != nil {
			panic(err)
		}
		span.AddEvent("load " + jslib.name)
	}

	return v8goCtx, func() {
		v8goCtx.Close()
		v8goIso.Dispose()
		h.v8goIsos <- v8go.NewIsolate()
	}
}

type jsInvokeReq struct {
	Script string `json:"script" binding:"required"`
	Data   any    `json:"data"`
}

type jsInvokeRes struct {
	Result any `json:"result"`
}

type jsInvokeErrRes struct {
	Error      string `json:"error"`
	Source     string `json:"source,omitempty"`
	StackTrace string `json:"stackTrace,omitempty"`
}

func (h *jsHandler) invoke(c *gin.Context) {
	span := trace.SpanFromContext(c)

	h.jsInvokeConcurrencyMetrics.Add(c, 1)
	defer h.jsInvokeConcurrencyMetrics.Add(c, -1)

	var req jsInvokeReq
	if strings.Contains(strings.ToLower(string(c.ContentType())), "application/json") {
		if err := c.ShouldBind(&req); err != nil {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusBadRequest, &jsInvokeErrRes{
				Error: err.Error(),
			})
			return
		}
	} else {
		req.Data, _ = readMultipartTextOrFile(c, "data")
		script, err := readMultipartTextOrFile(c, "script")
		if err != nil {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusBadRequest, &jsInvokeErrRes{
				Error: err.Error(),
			})
			return
		}
		req.Script = script
	}
	span.AddEvent("script", trace.WithAttributes(attribute.String("script", req.Script)))

	v8goCtx, close := h.generateV8GoContext(c)
	defer close()

	_, sspan := tracer.Start(c, "try set data to v8 global variable")
	if req.Data != nil {
		global := v8goCtx.Global()
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

	if _, err := v8goCtx.RunScript("try { data = JSON.parse(data) } catch {}", "parse.js"); err != nil {
		panic(err)
	}
	sspan.End()

	_, sspan = tracer.Start(c, "run javascript")
	result, err := v8goCtx.RunScript(req.Script, "script.js")
	if err != nil {
		if jsErr, ok := err.(*v8go.JSError); ok {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, &jsInvokeErrRes{
				Error:      jsErr.Message,
				Source:     jsErr.Location,
				StackTrace: jsErr.StackTrace,
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
		c.Error(errors.New("the output of script is null or undefined"))
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, &jsInvokeErrRes{
			Error: "The output of script is null or undefined, please make sure that the last line of your script contains the variables to be output.",
		})
		return
	}

	if result.IsObject() {
		c.JSON(http.StatusOK, &jsInvokeRes{
			Result: result.Object(),
		})
		return
	}

	c.JSON(http.StatusOK, &jsInvokeRes{
		Result: result.String(),
	})
}

func RegisterJSHandler(lc fx.Lifecycle, e *gin.Engine) error {
	jsInvokeConcurrencyUpDownCounter, err := meter.Int64UpDownCounter("js_invoke_concurrency", metric.WithDescription("Current concurrency of JavaScript invocation."))
	if err != nil {
		return err
	}

	jsConcurrency, _ := strconv.Atoi(os.Getenv("JS_CONCURRENCY"))
	if jsConcurrency <= 0 {
		jsConcurrency = 100
	}

	h := &jsHandler{
		v8goIsos: make(chan *v8go.Isolate, jsConcurrency),
		jslibs:   make([]*jsLibrary, 0),

		jsInvokeConcurrencyMetrics: jsInvokeConcurrencyUpDownCounter,
	}

	if err := h.loadJSLibrary(); err != nil {
		return err
	}

	for i := 0; i < jsConcurrency; i++ {
		h.v8goIsos <- v8go.NewIsolate()
	}

	js := e.Group("/js")
	{
		js.POST("/invoke", h.invoke)
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			for i := 0; i < jsConcurrency; i++ {
				v8goIso := <-h.v8goIsos
				v8goIso.Dispose()
			}
			return nil
		},
	})

	return nil
}
