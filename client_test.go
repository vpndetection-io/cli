package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/vpndetection-io/cli/lib"
)

// Both of the CLI's SDK clients, the lookup one and the keyless sign-in one, name
// the CLI first and the SDK after it, so the API can tell this CLI's traffic, and
// its version, from any other program built on the SDK.
func TestEveryRequestNamesTheCLIThenTheSDK(t *testing.T) {
	var mu sync.Mutex
	agents := map[string]string{}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		agents[r.URL.Path] = r.Header.Get("User-Agent")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/oauth/") {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "slow_down"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ip": strings.TrimPrefix(r.URL.Path, "/"), "is_vpn": false})
	}))
	t.Cleanup(api.Close)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedConfig, savedBaseURL := gConfig, fBaseURL
	savedNoCache, savedRetries := fNoCache, fRetries
	t.Cleanup(func() {
		gConfig, fBaseURL, fNoCache, fRetries = savedConfig, savedBaseURL, savedNoCache, savedRetries
	})
	gConfig, fBaseURL, fNoCache, fRetries = NewConfig(), api.URL, true, 0

	if _, err := captureStdout(t, func() error {
		return runLookup(t.Context(), []string{"9.1.0.1"}, lookupOpts{format: lib.FormatJSONL})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := startDeviceAuth(t.Context()); err == nil {
		t.Fatal("the sign-in start succeeded against an origin that refuses it")
	}

	want := "vpndetection-cli/" + version + " vpndetection-go/"
	if len(agents) != 2 {
		t.Fatalf("saw requests to %v, want one lookup and one sign-in start", agents)
	}
	for path, agent := range agents {
		if !strings.HasPrefix(agent, want) || len(agent) == len(want) || strings.Count(agent, " ") != 1 {
			t.Errorf("%s was sent User-Agent %q, want %s<the SDK's version>", path, agent, want)
		}
	}
}
