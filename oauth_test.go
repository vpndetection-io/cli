package main

import "testing"

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
		{"a session keeps its own deployment", "https://api-staging.vpndetection.io", "https://api-staging.vpndetection.io"},
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
