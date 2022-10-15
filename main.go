package main

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/ratelimit"
	"rogchap.com/v8go"
)

var (
	v8Isolate          *v8go.Isolate
	concurrencyLimiter chan struct{}
	rateLimiter        ratelimit.Limiter
)

func init() {
	v8Isolate = v8go.NewIsolate()
	concurrencyLimiter = make(chan struct{}, 10)
	rateLimiter = ratelimit.New(1000)
}

func readMultipartFile(c *gin.Context, key string) (string, error) {
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

func ping(c *gin.Context) {
	concurrencyLimiter <- struct{}{}
	defer func() {
		<-concurrencyLimiter
	}()

	rateLimiter.Take()

	ctx := v8go.NewContext(v8Isolate)
	defer ctx.Close()

	result, err := ctx.RunScript(`const message = 'pong'; message;`, "script.js")
	if err != nil {
		panic(err)
	}
	c.String(http.StatusOK, result.String())
}

func jsrun(c *gin.Context) {
	data, _ := readMultipartFile(c, "data")
	script, err := readMultipartFile(c, "script")
	if err != nil {
		panic(err)
	}

	concurrencyLimiter <- struct{}{}
	defer func() {
		<-concurrencyLimiter
	}()

	rateLimiter.Take()

	ctx := v8go.NewContext(v8Isolate)
	defer ctx.Close()

	if data != "" {
		global := ctx.Global()
		if dataValue, err := v8go.JSONParse(ctx, data); err != nil {
			if err := global.Set("data", data); err != nil {
				panic(err)
			}
		} else {
			if err := global.Set("data", dataValue); err != nil {
				panic(err)
			}
		}
	}

	result, err := ctx.RunScript(script, "script.js")
	if err != nil {
		if jsErr, ok := err.(*v8go.JSError); ok {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{
				"error":      jsErr.Message,
				"source":     jsErr.Location,
				"stackTrace": jsErr.StackTrace,
			})
			return
		}
		panic(err)
	}
	if result.IsObject() {
		c.JSON(http.StatusOK, result.Object())
		return
	}
	c.String(http.StatusOK, result.String())
}

func main() {
	defer v8Isolate.Dispose()
	r := gin.Default()
	r.GET("/", ping)
	r.POST("/", jsrun)
	r.Run()
}
