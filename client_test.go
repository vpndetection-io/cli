package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/vpndetection-io/cli/lib"
	vpndetection "github.com/vpndetection-io/sdk-go/v5"
)

// A lookup's 403 refuses a valid key for how it was used, and the spec's two
// reasons are the API's messages here. Neither is a plan or a license, so
// explain adds nothing to them.
func TestExplainLeavesALookup403ToTheAPI(t *testing.T) {
	for _, msg := range []string{
		"source address not allowed for this API key",
		"API key sent over plain HTTP; use https://",
	} {
		err := &vpndetection.Error{Kind: vpndetection.KindForbidden, Message: msg, StatusCode: 403}
		if got := explain(err).Error(); got != err.Error() {
			t.Errorf("explain(%q) = %q, want the SDK's error alone", msg, got)
		}
	}
}

// A spent allowance without a key is a spent keyless day, which `whoami` cannot show, since it
// needs a key: the hint names signing up instead.
func TestExplainSendsASpentKeylessCallerToSignUp(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "")
	t.Setenv("VPNDETECTION_SESSION", "")
	saved := gConfig
	t.Cleanup(func() { gConfig = saved })
	gConfig = NewConfig()

	err := &vpndetection.Error{Kind: vpndetection.KindQuotaExceeded, Message: "used up", StatusCode: 429}
	want := err.Error() + "\n`" + progBase + " signup` creates a free account and signs this machine in"
	if got := explain(err).Error(); got != want {
		t.Errorf("explain = %q, want %q", got, want)
	}
}

// A database call's license 403 keeps a hint saying where to see what the
// organization holds.
func TestExplainDatabaseNamesTheLicenseOnA403(t *testing.T) {
	for _, rc := range []string{"NOT_LICENSED", "LICENSE_EXPIRED"} {
		err := &vpndetection.Error{Kind: vpndetection.KindForbidden, Message: rc, StatusCode: 403}
		want := err.Error() + "\nthis organization's license does not cover that; `" + progBase +
			" db list` shows what it holds"
		if got := explainDatabase(err).Error(); got != want {
			t.Errorf("explainDatabase(%q) = %q, want %q", rc, got, want)
		}
	}
}

// A database call over plain HTTP is a 403 too, and naming the license there
// sent the caller to `db list`, which is refused the same way.
func TestExplainDatabaseLeavesANonLicense403ToTheAPI(t *testing.T) {
	err := &vpndetection.Error{Kind: vpndetection.KindForbidden, Message: "KEY_OVER_HTTP", StatusCode: 403}
	if got := explainDatabase(err).Error(); got != err.Error() {
		t.Errorf("explainDatabase(KEY_OVER_HTTP) = %q, want the SDK's error alone", got)
	}
}

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
