# jn

## Try it for free !
https://kn.weii.dev/default/jn

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
```bash
# json
curl -s -X POST -H 'Content-Type: application/json' http://localhost:8080/invoke/js \
--data-raw '{
    "script": "data.map(x => x * 2)",
    "data": [1,2]
}' | jq

# or martipart form
echo 'data.map(x => x * 2)' > script.js
curl -s -X POST -F 'script="data.map(x => x * 2)"' -F 'data="[1,2]"' http://localhost:8080/invoke/js | jq

# or martipart form file
echo 'data.map(x => x * 2)' > script.js
echo '[1,2]' > data.json
curl -s -X POST -F 'script=@"./script.js"' -F 'data=@"./data.json"' http://localhost:8080/invoke/js | jq

# or martipart form text and file
echo 'data.map(x => x * 2)' > script.js
curl -s -X POST -F 'script=@"./script.js"' -F 'data="[1,2]"' http://localhost:8080/invoke/js | jq
```
```json
{
  "result": [
    2,
    4
  ]
}
```

## Embaded some JavaScript Libraries
### Lodash
https://lodash.com
```bash
curl -s -X POST -H 'Content-Type: application/json' http://localhost:8080/invoke/js \
--data-raw '{
    "script":"_.defaults({ '\''a'\'': 1 }, { '\''a'\'': 3, '\''b'\'': 2 });"
}' | jq
```
```json
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
curl -s -X POST -H 'Content-Type: application/json' http://localhost:8080/invoke/js \
--data-raw '{
    "script":"moment().format('\''MMMM Do YYYY, h:mm:ss a'\'')"
}' | jq
```
```json
{
  "result": "December 10th 2022, 4:34:42 am"
}
```

### base64.js
https://github.com/mathiasbynens/base64
```bash
curl -s -X POST -H 'Content-Type: application/json' http://localhost:8080/invoke/js \
--data-raw '{
    "script": "base64.encode('\''abc'\'')"
}' | jq
```
```json
{
  "result": "YWJj"
}
```
