# jsrun

A server run javascript with input and output the results.

**Request**
```json
{
    "data": {
        "a": "cccc"
    },
    "script": "const result = data; [result].map(item => item.a);"
}
```

**Response**
```
{
    "result": [
        "cccc"
    ]
}
```

## How to build?
```bash
pack build --builder=gcr.io/buildpacks/builder:v1 --publish wei840222/jsrun:10
```

## How to deploy?
```bash
kn ksvc apply --image=wei840222/jsrun:10 jsrun
```

## How to deploy by tekton?
```bash
kubectl apply -k .tekton
```