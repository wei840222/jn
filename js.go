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
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/ratelimit"
	v8 "rogchap.com/v8go"
)

//go:embed jslib
var jslib embed.FS

type jsHandler struct {
	v8Iso *v8.Isolate
	cl    chan struct{}
	rl    ratelimit.Limiter
}

func (h *jsHandler) allowLimit() func() {
	h.cl <- struct{}{}
	h.rl.Take()
	return func() {
		<-h.cl
	}
}

func (h *jsHandler) invoke(c *gin.Context) {
	done := h.allowLimit()
	defer done()

	span := trace.SpanFromContext(c)
	span.AddEvent("allowLimit", trace.WithAttributes(attribute.Int("concurrency", len(h.cl))))

	var req struct {
		Script string `json:"script" binding:"required"`
		Data   any    `json:"data"`
	}
	if err := c.Bind(&req); err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	span.AddEvent("script", trace.WithAttributes(attribute.String("script", req.Script)))

	ctx := v8.NewContext(h.v8Iso)
	defer ctx.Close()

	if req.Data != nil {
		global := ctx.Global()
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

	if _, err := ctx.RunScript("try { data = JSON.parse(data) } catch {}", "parse.js"); err != nil {
		panic(err)
	}

	_, sspan := tracer.Start(c, "Load JavaScript Library")
	if err := fs.WalkDir(jslib, ".", func(path string, entry fs.DirEntry, err error) error {
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
			if _, err := ctx.RunScript(string(b), info.Name()); err != nil {
				return err
			}
			sspan.AddEvent("load jslib", trace.WithAttributes(attribute.String("name", info.Name())))
		}
		return nil
	}); err != nil {
		panic(err)
	}
	sspan.End()

	_, sspan = tracer.Start(c, "Run JavaScript")
	result, err := ctx.RunScript(req.Script, "script.js")
	if err != nil {
		if jsErr, ok := err.(*v8.JSError); ok {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
				"error":      jsErr.Message,
				"source":     jsErr.Location,
				"stackTrace": jsErr.StackTrace,
			})
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
	h := &jsHandler{
		v8Iso: v8.NewIsolate(),
		cl:    make(chan struct{}, 10),
		rl:    ratelimit.New(1000),
	}

	e.POST("/invoke/js", h.invoke)

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			h.v8Iso.Dispose()
			return nil
		},
	})

	return nil
}
