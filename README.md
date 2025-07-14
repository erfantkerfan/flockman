## About Flockman

Flockman is tool designed to help DevOps with a simple tool to rollout updates to a swarm cluster using basic rest APIs. some of its features are:

- Secure and Simple
- written in [GO](https://go.dev/)
- small packaged binary for starting server and also a cli tool for management purposes
- appends current image tag to environment variables inside the container with the key of `FLOCKMAN_IMAGE_TAG`

## Learning Flockman

best way to start with Flockman is to [download](https://github.com/erfantkerfan/flockman/releases) the latest binary and start using its cli and figuring out its capabilities yourself.

## api documentation

<details>
  <summary> get node details </summary>

    GET `/api/v1/node`

```json
{
  "node_name":"erfan-zenbook-ux325ea"
}
```

</details>


<details>
  <summary> get service status </summary>


```json
POST `/api/v1/service/status`

{
    "token":"TOKEN"
}
```


```json
{
    "image":"nginx:latest","service":"nginx"
}
```

</details>


<details>
  <summary> update service status </summary>


```json
POST `/api/v1/service/update`

{
    "token":"TOKEN",
    "tag":"alpine",
    "start_first":true,
    "stop_signal":"QUIT"
}
```


```json
{
    "image":"nginx:alpine",
    "service":"nginx"
}
```

</details>
