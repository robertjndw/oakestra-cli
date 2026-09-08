# oakestra-go

A Go client library for the [Oakestra](https://oakestra.io) System Manager API, in the
style of [`google/go-github`](https://github.com/google/go-github): a `Client` with one
service per resource, `context.Context`-first methods, functional options, and typed
errors.

This library is config-agnostic - it never reads files or environment variables. The
caller supplies a base URL and credentials. The [Oakestra CLI](../oak_go_cli) is built on
top of it and is a good reference for a real consumer.

## Install

```sh
go get github.com/oakestra/oakestra-cli/oakestra-go
```

## Usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/oakestra/oakestra-cli/oakestra-go"
)

func main() {
	client, err := oakestra.NewClient(
		oakestra.WithBaseURL(oakestra.BaseURLForHost("127.0.0.1")),
		oakestra.WithBasicLogin("Admin", "Admin"),
	)
	if err != nil {
		log.Fatal(err)
	}

	apps, _, err := client.Applications.List(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, app := range apps {
		fmt.Println(app.ApplicationName, app.ApplicationID)
	}
}
```

See [`examples/list-apps`](examples/list-apps) for a runnable version.

## Authentication

- `WithBasicLogin(username, password)` - logs in against `/api/auth/login` and caches
  the token (matching the Oakestra CLI's behaviour). Cache lifetime defaults to
  `DefaultTokenTTL` and can be overridden with `WithTokenTTL`.
- `WithToken(token)` - skips the login flow for a token you already hold.
- `WithAuth(provider)` - plug in any `AuthProvider` for custom schemes (OIDC, a token
  sourced from a file you manage, etc.).

## Errors

Failed calls return one of:

- `*oakestra.ErrorResponse` - a non-2xx response that isn't an auth failure
- `*oakestra.AuthError` - login rejected, or a request failed with 401/403
- `*oakestra.ConnectionError` - the base URL was unreachable (wraps the network error)
- `*oakestra.NotFoundError` / `*oakestra.MultipleMatchesError` - from the `ResolveByNameOrID`
  / `FindByNameOrID` helpers

Use `errors.As` to react to a specific case.

## Endpoints without a typed method

`Client.NewRequest` and `Client.Do` are exported, so you can reach any System Manager
endpoint this package doesn't wrap yet while still getting request signing, JSON
decoding (including Oakestra's occasional JSON-encoded-string bodies), and error
translation for free.

## Scope

v0.1 covers authentication, applications, services, and clusters - the System Manager
surface the Oakestra CLI uses today. Cluster-manager endpoints (port 10100), addon APIs
(e.g. FLOps), retries, and pagination are not covered; use the escape hatch above until
there's a typed method for them.
