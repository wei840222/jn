# jn

## Try it for free !
https://knative.weii.dev/default/jn

## Why name jn ?
"J"SON Functio"n"s, "J"SO"N"

A server provide many function as RESTful API for processing JSON input and output the results.

## How to ?

### Install development tools
```bash
go install github.com/silenceper/gowatch@latest
```

### Build
```bash
pack build --builder=gcr.io/buildpacks/builder:v1 --publish wei840222/jn:2
```

### Deploy
```bash
kn ksvc apply --annotation=prometheus.io/scrape=true --annotation=prometheus.io/port=2222 --annotation=instrumentation.opentelemetry.io/inject-sdk=true --image=wei840222/jn:2 jn
```

### Deploy by tekton
```bash
kubectl apply -k .tekton
```
