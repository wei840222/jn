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
		fx.Provide(InitMeterProvider),
		fx.Provide(InitTracerProvider),
		fx.Provide(InitGinEngine),
		fx.Invoke(RegisterJSHandler),
	).Run()
}
