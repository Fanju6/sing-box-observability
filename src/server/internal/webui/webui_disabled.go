//go:build !webui

package webui

import "net/http"

// Handler leaves API-only development builds unchanged. Production builds use
// the webui tag after the frontend distribution has been copied beside the
// tagged embed implementation.
func Handler(api http.Handler) http.Handler { return api }
