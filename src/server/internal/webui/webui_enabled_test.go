//go:build webui

package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedHandlerServesSPAAndPreservesAPI(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("api"))
	})
	handler := Handler(api)

	apiResponse := httptest.NewRecorder()
	handler.ServeHTTP(apiResponse, httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil))
	if apiResponse.Code != http.StatusTeapot || apiResponse.Body.String() != "api" {
		t.Fatalf("API request was not preserved: status=%d body=%q", apiResponse.Code, apiResponse.Body.String())
	}

	spaResponse := httptest.NewRecorder()
	handler.ServeHTTP(spaResponse, httptest.NewRequest(http.MethodGet, "/connections", nil))
	if spaResponse.Code != http.StatusOK || !strings.Contains(spaResponse.Body.String(), `<div id="root"></div>`) {
		t.Fatalf("SPA fallback failed: status=%d body=%q", spaResponse.Code, spaResponse.Body.String())
	}
	if spaResponse.Header().Get("X-Content-Type-Options") != "nosniff" || spaResponse.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("static security headers missing: %#v", spaResponse.Header())
	}
}
