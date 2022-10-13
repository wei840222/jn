package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
	"rogchap.com/v8go"
)

//go:embed web/build
var webFiles embed.FS
var webHandler http.Handler
var v8Iso *v8go.Isolate
var limiter chan struct{}

func init() {
	sub, err := fs.Sub(webFiles, "web/build")
	if err != nil {
		panic(err)
	}
	webHandler = http.FileServer(http.FS(sub))
	v8Iso = v8go.NewIsolate()
	limiter = make(chan struct{}, 2)
}

func web(c *gin.Context) {
	webHandler.ServeHTTP(c.Writer, c.Request)
}

func ping(c *gin.Context) {
	limiter <- struct{}{}
	defer func() {
		<-limiter
	}()
	ctx := v8go.NewContext(v8Iso)
	defer ctx.Close()
	result, err := ctx.RunScript(`const message = 'pong'; message;`, "script.js")
	if err != nil {
		panic(err)
	}
	c.String(http.StatusOK, result.String())
}

func jsrun(c *gin.Context) {
	limiter <- struct{}{}
	defer func() {
		<-limiter
	}()
	var request struct {
		Script string `json:"script" binding:"required"`
		Data   any    `json:"data"`
	}
	if err := c.Bind(&request); err != nil {
		if err != nil {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
	}

	ctx := v8go.NewContext(v8Iso)
	defer ctx.Close()

	if request.Data != nil {
		global := ctx.Global()
		b, err := json.Marshal(request.Data)
		if err != nil {
			panic(err)
		}
		if err := global.Set("data", string(b)); err != nil {
			panic(err)
		}
	}

	if _, err := ctx.RunScript("try{data=JSON.parse(data)}catch{}", "parse.js"); err != nil {
		panic(err)
	}
	result, err := ctx.RunScript(request.Script, "main.js")
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": result.Object(),
	})
}

func main() {
	defer v8Iso.Dispose()
	r := gin.Default()
	r.GET("/", web)
	r.GET("/static/*file", web)
	r.NoRoute(web)
	r.POST("/", jsrun)
	r.GET("/ping", ping)
	r.Run()
}