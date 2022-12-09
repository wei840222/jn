package main

import (
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/otel"
	"go.uber.org/fx"
)

var (
	json   = jsoniter.ConfigCompatibleWithStandardLibrary
	tracer = otel.Tracer("github.com/wei840222/jn")
)

func main() {
	fx.New(
		fx.Provide(InitMeterProvider),
		fx.Provide(InitTracerProvider),
		fx.Provide(InitGinEngine),
		fx.Invoke(RegisterJSHandler),
	).Run()
}
