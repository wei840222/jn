package main

import (
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

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

func InitGinEngine(lc fx.Lifecycle) *gin.Engine {
	e := gin.Default()
	e.ContextWithFallback = true

	e.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: e,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
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
