package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	ginprom "github.com/wei840222/gin-prometheus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

var ginOtelLogFormatter = func(param gin.LogFormatterParams) string {
	var statusColor, methodColor, resetColor string
	if param.IsOutputColor() {
		statusColor = param.StatusCodeColor()
		methodColor = param.MethodColor()
		resetColor = param.ResetColor()
	}

	if param.Latency > time.Minute {
		param.Latency = param.Latency.Truncate(time.Second)
	}

	return fmt.Sprintf("[GIN] %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v traceID=%s\n%s",
		param.TimeStamp.Format("2006/01/02 - 15:04:05"),
		statusColor, param.StatusCode, resetColor,
		param.Latency,
		param.ClientIP,
		methodColor, param.Method, resetColor,
		param.Path,
		trace.SpanContextFromContext(param.Request.Context()).TraceID(),
		param.ErrorMessage,
	)
}

func readMultipartTextOrFile(c *gin.Context, key string) (string, error) {
	if text := c.PostForm(key); text != "" {
		return text, nil
	}
	fileHeader, err := c.FormFile(key)
	if err != nil {
		return "", err
	}
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func InitGinEngine(lc fx.Lifecycle, tp trace.TracerProvider, otelpromExporter *otelprom.Exporter) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.ContextWithFallback = true
	p := ginprom.NewPrometheus("gin").SetEnableExemplar(true).SetOtelPromExporter(otelpromExporter)
	e.Use(otelgin.Middleware("gin", otelgin.WithTracerProvider(tp)), p.HandlerFunc(), gin.LoggerWithFormatter(ginOtelLogFormatter), gin.Recovery())

	e.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: e,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			p.SetListenAddress(":2222").SetMetricsPath(nil)
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					panic(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})

	return e
}
