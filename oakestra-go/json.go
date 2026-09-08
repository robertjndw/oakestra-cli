package oakestra

import "encoding/json"

// smartUnmarshal handles both a direct JSON body and a JSON-encoded string.
// Some Oakestra versions wrap the response body in a JSON string (e.g. the
// body is `"[{...}]"` instead of `[{...}]`), so a plain json.Unmarshal
// fails against those deployments.
func smartUnmarshal(data []byte, v any) error {
	// Fast path: direct unmarshal.
	if err := json.Unmarshal(data, v); err == nil {
		return nil
	}
	// The API may have returned a JSON-encoded string, so unwrap and retry.
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		// Neither approach worked; surface the original parse error.
		return json.Unmarshal(data, v)
	}
	return json.Unmarshal([]byte(s), v)
}
