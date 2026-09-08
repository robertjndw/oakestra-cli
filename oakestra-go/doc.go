/*
Package oakestra is a Go client library for the Oakestra System Manager API.

# Usage

Construct a client with NewClient, then use the service on Client that
matches the resource you want (Applications, Services, Clusters):

	c, err := oakestra.NewClient(
	    oakestra.WithBaseURL("http://127.0.0.1:10000"),
	    oakestra.WithBasicLogin("Admin", "Admin"),
	)
	if err != nil {
	    log.Fatal(err)
	}

	apps, _, err := c.Applications.List(context.Background())

# Authentication

WithBasicLogin logs in against /api/auth/login and caches the resulting
token, matching how the Oakestra CLI authenticates. WithToken skips the
login flow for callers that already hold a token. WithAuth accepts any
AuthProvider for custom schemes.

# Errors

Failed requests return one of ErrorResponse, AuthError, ConnectionError,
NotFoundError, or MultipleMatchesError - inspect with errors.As to react to
a specific failure mode.

# Endpoints without a typed method

Client.NewRequest and Client.Do are exported so callers can reach any
System Manager endpoint this package does not yet wrap in a typed method,
while still getting request signing, JSON decoding, and error translation.
*/
package oakestra
