package main

import (
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/fx"
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

func main() {
	fx.New(
		fx.Provide(InitGinEngine),
		fx.Invoke(RegisterRapiDocHandler),
		fx.Invoke(RegisterJSHandler),
	).Run()
}
