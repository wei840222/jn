# jn

## Why name jn ?
**J**SON Functio**n**s, **J**SO**N**

A server run provide many function as RESTful API for processing JSON input and output the results.

## How to build?
```bash
pack build --builder=gcr.io/buildpacks/builder:v1 --publish wei840222/jn:2
```

## How to deploy?
```bash
kn ksvc apply --annotation=prometheus.io/scrape=true --annotation=prometheus.io/port=2222 --annotation=instrumentation.opentelemetry.io/inject-sdk=true --image=wei840222/jn:2 jn
```

## How to deploy by tekton?
```bash
kubectl apply -k .tekton
```

## Example Usage
### Request
```bash
curl -X POST -F 'data=@/Users/wei840222/Desktop/data.json' -F 'script=@/Users/wei840222/Desktop/script.js' http://localhost:8080/invoke/js
```
or
```bash
curl -X POST -H 'Content-Type: application/json' http://localhost:8080/invoke/js \
--data-raw '{
    "script": "data.flat().map(x => x * 2).filter(x => x > 5)",
    "data": [
        [1,2,3,4,5],
        [6,7,8,9]
    ]
}'
```

### Response
```
{
    "result": [6,8,10,12,14,16,18]
}
```

## Embaded some JavaScript Libraries
### Lodash
https://lodash.com
```bash
curl -X POST -H 'Content-Type: application/json' http://localhost:8080/invoke/js \
--data-raw '{
    "script":"_.defaults({ '\''a'\'': 1 }, { '\''a'\'': 3, '\''b'\'': 2 });"
}'
```
```
{
    "result": {
        "a": 1,
        "b": 2
    }
}
```

### Moment.js
https://momentjs.com
```bash
curl -X POST -H 'Content-Type: application/json' http://localhost:8080/invoke/js \
--data-raw '{
    "script":"moment().format('\''MMMM Do YYYY, h:mm:ss a'\'')"
}'
```
```
{
    "result": "October 19th 2022, 3:57:56 pm"
}
```

### base64.js
https://github.com/mathiasbynens/base64
```bash
curl -X POST -H 'Content-Type: application/json' http://localhost:8080/invoke/js \
--data-raw '{
    "script": "base64.encode('abc')"
}'
```
```
{
    "result": "YWJj"
}
```
