# YakAPI Server

The YakAPI Server is meant to run on Rovers to provide an API for it. It is
extensible by allowing implementation specific software to participate in a
Rover software ecosystem using Redis as middleware.

## Usage

`yakapi` acts as a http server that presents a JSON API

### API

The JSON API is versioned, so start with `/v1` and you'll receive a nice hello:

```ShellSession
$ curl -s http://localhost:8080/v1 | jq .
{
  "name": "YakAPI (development)",
  "uptime": 1,
  "resources": [
    {
      "name": "metrics",
      "ref": "/metrics"
    },
    {
      "name": "ci",
      "ref": "/v1/ci"
    },
    {
      "name": "cam",
      "ref": "/v1/cam/capture"
    },
    {
      "name": "project",
      "ref": "https://github.com/rhettg/batteries/yakapi"
    }
  ]
}
```

### Services

YakAPI requires additional services to provide actual functionality. It
communicates with these services via Redis Streams. An example implementation is
provided in [`examples/ci.py`](examples/ci.py).

### Development

YakAPI lives by [scripts-to-rule-them-all](https://github.com/github/scripts-to-rule-them-all) rules.

But dependencies and setup are limited. Really you should be able to go out of the box with:

```ShellSession
$ script/server
2022-05-13T03:09:53.020Z        INFO    yakapi/main.go:218      starting        {"version": "1.0.0", "port": "8080"}
```

### Production

Sky is the limit, but for easy integration a `Dockerfile` is provided that is easily customized by environment variables.

```ShellSession

$ docker build -f Dockerfile -t yakapi:latest .
...
$ docker run --rm \
  -p 80:8080 \
  -e YAKAPI_NAME="My Rover" \
  -e YAKAPI_REDIS_URL="127.0.0.1:6379" \
  -e YAKAPI_CAM_CAPTURE_PATH="/var/cam/capture.jpeg" \
  -v /var/cam:/var/cam \
 yakapi:latest 
...
```

### Configuration

Configuration is primarily through environment variables

* `YAKAPI_PORT` [default `8080`] port for api server to listen on
* `YAKAPI_NAME` [default `YakBot`] name for rover 
* `YAKAPI_REDIS_URL` [default `redis://localhost:6379`] URL for redis server
* `YAKAPI_PROJECT_URL` [default `https://github.com/The-Yak-Collective/yakrover`] URL for more information
* `YAKAPI_CAM_CAPTURE_PATH` path to image for camera.

## Components

### Metrics

Metrics are served in [prometheus](https://prometheus.io) format:

```ShellSession
$ curl -s http://localhost:8080/metrics
# HELP yakapi_processed_ops_total The total number of processed requests
# TYPE yakapi_processed_ops_total counter
yakapi_processed_ops_total 0
...

```

### ci (command injection)

This service translates commands into motor settings. There are two redis streams that a implementation must interact with:

* `yakapi:ci` stream of accepted commands
* `yakapi:ci:result` results of executing commands (XADD with the same id as the command)

### cam

The camera component can current serve an image if placed in a configured path.

**TODO**: Redis-ify
