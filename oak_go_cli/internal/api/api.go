// Package api adapts the oakestra-go client library to the CLI's local
// config file and adds the "oak config ..." remediation hints that the
// library itself deliberately doesn't know about.
package api

import (
	"errors"
	"fmt"
	"strings"

	oakestra "github.com/oakestra/oakestra-cli/oakestra-go"

	"github.com/oakestra/oak-go-cli/internal/config"
)

// New builds an oakestra-go client from the local CLI config file
// (~/oak_cli/.oak_go_cli_config.json).
func New() (*oakestra.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return oakestra.NewClient(
		oakestra.WithBaseURL(cfg.URL()),
		oakestra.WithBasicLogin(cfg.GetUsername(), cfg.GetPassword()),
		oakestra.WithUserAgent("oak-cli"),
	)
}

// Hint wraps a library error with the CLI-specific remediation the library
// can't offer on its own: which local `oak config` command fixes it. Errors
// it doesn't recognize are returned unchanged.
func Hint(err error) error {
	if err == nil {
		return nil
	}

	var connErr *oakestra.ConnectionError
	if errors.As(err, &connErr) {
		// WithBaseURL normalizes a trailing slash onto the URL; strip it back
		// off so this reads the way the CLI's own config values do.
		baseURL := strings.TrimSuffix(connErr.BaseURL, "/")
		return fmt.Errorf(
			"root orchestrator not reachable at %s\n"+
				"Make sure Oakestra is running, then configure the address with:\n"+
				"  oak config set system_manager_ip <IP>\n"+
				"(underlying error: %v)",
			baseURL, connErr.Err,
		)
	}

	var authErr *oakestra.AuthError
	if errors.As(err, &authErr) {
		// Username/target are only for display, so fall back quietly if the
		// config can't be loaded rather than hiding the real auth error.
		username := "Admin"
		target := ""
		if cfg, cfgErr := config.Load(); cfgErr == nil {
			username = cfg.GetUsername()
			target = cfg.URL()
		}
		return fmt.Errorf(
			"login failed (HTTP %d): %s\n"+
				"If the Root Orchestrator is running on a different host, update the address with:\n"+
				"  oak config set system_manager_ip <IP>\n"+
				"(current target: %s)\n"+
				"Credentials: %s / ***\n"+
				"Update with: oak config credentials <username> [password]",
			authErr.StatusCode, authErr.Body, target, username,
		)
	}

	var errResp *oakestra.ErrorResponse
	if errors.As(err, &errResp) {
		return fmt.Errorf("%s %s returned HTTP %d: %s", errResp.Method, errResp.URL, errResp.StatusCode, errResp.Body)
	}

	return err
}
