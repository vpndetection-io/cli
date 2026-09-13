package main

import "testing"

// The console URL is derived from the session's API host rather than listed,
// so a new deployment needs no code change here. A host we do not recognise
// must fall back to production rather than to a guess: sending someone to a
// made-up console is worse than sending them to the real one.
func TestSignupURL(t *testing.T) {
	t.Cleanup(func() { gConfig = NewConfig(); fBaseURL = "" })

	for _, tt := range []struct {
		base, want string
	}{
		{"", "https://app.vpndetection.io/auth/signup"},
		{"https://api.vpndetection.io", "https://app.vpndetection.io/auth/signup"},
		{"https://api-dev.vpndetection.io", "https://app-dev.vpndetection.io/auth/signup"},
		{"https://api.example.com", "https://app.vpndetection.io/auth/signup"},
		{"https://vpndetection.io", "https://app.vpndetection.io/auth/signup"},
		{"://broken", "https://app.vpndetection.io/auth/signup"},
	} {
		gConfig = NewConfig()
		fBaseURL = tt.base
		if got := signupURL(); got != tt.want {
			t.Errorf("base %q: got %q, want %q", tt.base, got, tt.want)
		}
	}
}
