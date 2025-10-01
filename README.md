# jn

## Try it for free !

https://jn.weii.dev

## Why name jn ?

"J"SON Functio"n"s, "J"SO"N"

A server provide many function as RESTful API for processing JSON input and output the results.

## How to deploy ?

```sh
kn service apply jn --image=gitea.tailb0283.ts.net/wei840222/jn:main --annotation-service=home-infra.cloudflare/domain=jn.weii.dev --request="cpu=125m,memory=256Mi" --limit="cpu=500m,memory=512Mi" --port=8080 --probe-readiness="http::8080:/health" --probe-liveness="http::8080:/health"
```
