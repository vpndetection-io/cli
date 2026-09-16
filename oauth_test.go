package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	vpndetection "github.com/vpndetection-io/sdk-go/v5"
)

// The authorization server is mounted on PATHS of the API host, so this is the
// API base URL and nothing else. The earlier shape derived an `auth.<brand>`
// hostname from it by string surgery, which is exactly the sort of thing that
// works in production and silently points at the wrong deployment everywhere
// else - so these cases exist to keep the derivation from creeping back.
func TestAuthBaseURL(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{"unset falls back to production", "", "https://api.vpndetection.io"},
		// A hyphenated first label is the shape a non-production deployment
		// takes, and the old code rewrote exactly that away. Kept generic: this
		// repo is public.
		{"a session keeps its own deployment", "https://api-eu.example.com", "https://api-eu.example.com"},
		{"a trailing slash does not double up", "https://api.example.com/", "https://api.example.com"},
		{"an unrelated host is honoured, not rewritten", "https://api.example.com", "https://api.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saveFlag, saveCfg := fBaseURL, gConfig
			// ResolveBaseURL reads the flag first, which is the precedence a
			// `--base-url` on the command line relies on.
			fBaseURL = tt.base
			gConfig = Config{Sessions: map[string]*Session{}}
			defer func() { fBaseURL, gConfig = saveFlag, saveCfg }()

			if got := authBaseURL(); got != tt.want {
				t.Errorf("authBaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// What a person reads when the server refuses a sign-in, and a revoke the
// server failed reported as failed rather than read as success.
func TestSignInRefusals(t *testing.T) {
	dev := &vpndetection.DeviceAuthorization{DeviceCode: "mo_dc_x", ExpiresIn: 900, Interval: 1}
	start := func(ctx context.Context) error {
		_, err := startDeviceAuth(ctx)
		return err
	}
	poll := func(ctx context.Context) error {
		_, err := pollForToken(ctx, dev)
		return err
	}
	revoke := func(ctx context.Context) error {
		return revokeToken(ctx, "mo_rt_x")
	}
	tests := []struct {
		name   string
		status int
		answer string
		call   func(context.Context) error
		want   string
	}{
		{"slow_down refuses the start", 400, `{"error":"slow_down"}`, start,
			"too many sign-in attempts from this address; wait a minute and try again"},
		{"access_denied ends the poll", 400, `{"error":"access_denied"}`, poll,
			"the request was denied in the browser"},
		{"expired_token ends the poll", 400, `{"error":"expired_token"}`, poll,
			"the code expired before it was approved; run the command again"},
		{"a revoke answered 500 fails", 500, `{"rc":"ERROR"}`, revoke, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.answer)
			}))
			defer server.Close()
			saveFlag, saveRetries, saveCfg := fBaseURL, fRetries, gConfig
			fBaseURL, fRetries = server.URL, 0
			gConfig = Config{Sessions: map[string]*Session{}}
			defer func() { fBaseURL, fRetries, gConfig = saveFlag, saveRetries, saveCfg }()
			// A poll that mistook the answer for pending would otherwise never end.
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			err := tt.call(ctx)
			if err == nil {
				t.Fatal("succeeded, want an error")
			}
			if tt.want != "" && err.Error() != tt.want {
				t.Errorf("error was %q, want %q", err, tt.want)
			}
		})
	}
}
