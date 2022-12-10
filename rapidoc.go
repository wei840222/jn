package main

import (
	"embed"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"
)

//go:embed api/*
var apiFiles embed.FS

// @title           jn
// @version         0.0.1
// @description     A server provide many function as RESTful API for processing JSON input and output the results.

// @contact.name   wei840222
// @contact.email  wei840222@gmail.com

// @license.name  MIT
// @license.url   https://github.com/wei840222/jn/blob/main/LICENSE

//go:generate swag i --output=./api --outputTypes=json --generalInfo=./rapidoc.go
func RegisterRapiDocHandler(e *gin.Engine) {
	e.GET("/", func(c *gin.Context) {
		b, err := apiFiles.ReadFile("api/index.html")
		if err != nil {
			panic(err)
		}
		c.Data(http.StatusOK, "text/html", b)
	})
	e.GET("/swagger.json", func(c *gin.Context) {
		b, err := apiFiles.ReadFile("api/swagger.json")
		if err != nil {
			panic(err)
		}
		b, err = sjson.SetBytes(b, "servers", []struct {
			URL         string `json:"url"`
			Description string `json:"description"`
		}{
			{
				URL:         "http://localhost:8080",
				Description: "Local",
			},
			{
				URL:         "https://kn.weii.dev/default/jn",
				Description: "Production",
			},
		})
		if err != nil {
			panic(err)
		}
		c.Data(http.StatusOK, "application/json", b)
	})
}
