# preflight-dns

a preflight check for DNS migrations. This tool will check the current user-facing connection state, then retry the connection using the proposed new DNS. If it exits with a non-zero exit code, stop and review the configuration before proceeding with the DNS migration.

## Building

Note: the following commands assume you have a working Go environment. See the [install instructions](https://golang.org/doc/install) if you need to install Go.

```bash
make
```

## Install

NOTE: you will need `curl`, `bash`, and `jq` installed for the install script to work. It will attempt to install the binary in `/usr/local/bin` and will require `sudo` access. You can override the install directory by setting the `INSTALL_DIR` environment variable.

```bash
curl -sSL https://raw.githubusercontent.com/robertlestak/preflight-dns/main/scripts/install.sh | bash
```

## Usage

```bash
~ preflight-dns -h
Usage of preflight-dns:
  -body string
        body to send
  -endpoint string
        endpoint to check
  -headers string
        headers to send. comma separated list of key=value
  -lib
        lower is better. default is exact status code match.
  -log-level string
        log level (default "info")
  -method string
        method to use (default "GET")
  -new string
        new hostname/ip to use
  -server
        run in server mode
  -server-addr string
        server address to listen on (default ":8080")
  -timeout duration
        timeout for requests (default 5s)
```

### Simple example

```bash
preflight-dns \
	-endpoint https://example.com \
	-new new-example.us-east-1.elb.amazonaws.com
```

### Example with headers and body

```bash
preflight-dns \
	-endpoint https://example.com \
	-new new-example.us-east-1.elb.amazonaws.com \
	-headers "Authorization=Bearer 1234,Hello=World" \
	-body '{"hello": "world"}'
```

### Example with lower is better mode

By default, `preflight-dns` will expect an exact status code match between the current endpoint and the new endpoint. 

If you want to use a lower is better mode, you can use the `-lib` flag. This will return a non-zero exit code if the new endpoint returns a status code that is lower than or equal to the current endpoint.

```bash
preflight-dns \
	-endpoint https://example.com \
	-new new-example.us-east-1.elb.amazonaws.com \
	-lib
```

### Server mode

When run in `server` mode, `preflight-dns` will listen on the specified address and port and respond to requests with the current connection state. This is useful for running as a service and calling as part of a CI/CD pipeline.

```bash
preflight-dns -server -server-addr :8080
```

```bash
curl localhost:8080/ -d '{
	"endpoint": "https://example.com",
	"new": "new-example.us-east-1.elb.amazonaws.com",
	"headers": {
		"hello": "world"
	},
	"method": "POST",
	"body": "hello world"
}'
```

#### Payload

The payload for the server mode is a JSON object with the following fields:

| Field    | Type   | Description                                                                 |
| -------- | ------ | --------------------------------------------------------------------------- |
| endpoint | string | the endpoint to check                                                       |
| new      | string | the new hostname/ip to use                                                  |
| headers  | object | map of headers to send. keys are header names, values are header values     |
| method   | string | the HTTP method to use (default: GET)                                       |
| body     | string | the body to send with the request (default: empty)                          |
| timeout  | int    | the timeout for the request in nano seconds (default: 5000000000)           |

#### Response

On success, the server will respond with a `200 OK` and no body.

On failure, the server will respond with a `500 Internal Server Error` and an error message string.

## Package

This package can be imported and used in your own code.

```go
package main

import (
	"log"
	"time"

	"github.com/robertlestak/preflight-dns/pkg/preflightdns"
)

func main() {
	pf := &preflightdns.PreflightDNS{
		Endpoint: "https://example.com",
		Method:   "GET",
		New:      "new-example.us-east-1.elb.amazonaws.com",
		Timeout:  time.Duration(5 * time.Second),
	}
	if err := pf.Run(); err != nil {
		log.Fatal(err)
	}
}
```