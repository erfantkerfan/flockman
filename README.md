## About Flockman

Flockman is a tool designed to help DevOps with a simple way to rollout updates to a swarm cluster using basic REST APIs. Some of its features are:

- Secure and Simple
- written in [GO](https://go.dev/)
- small packaged binary for starting server and also a cli tool for management purposes
- appends current image tag to environment variables inside the container with the key of `FLOCKMAN_IMAGE_TAG`

## Learning Flockman

best way to start with Flockman is to [download](https://github.com/erfantkerfan/flockman/releases) the latest binary and start using its cli and figuring out its capabilities yourself.

## Prometheus metrics

Metrics are disabled by default. Pass `--metrics-port` (or `-M`) to start a separate exporter listener. The metrics port must differ from `--port`.

```bash
./flockman serve --port 8314 --metrics-port 8315
curl http://127.0.0.1:8315/metrics
```

Example Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: flockman
    static_configs:
      - targets: ["127.0.0.1:8315"]
```

### Grafana dashboard

Import [`deploy/grafana/flockman-dashboard.json`](deploy/grafana/flockman-dashboard.json) via **Dashboards → New → Import**, or provision it from that path.

The dashboard uses Prometheus template variables (`datasource`, `job`, `instance`) and `$__rate_interval` for rate/histogram queries. Pick your Prometheus datasource after import if Grafana does not resolve `${datasource}` automatically.

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
