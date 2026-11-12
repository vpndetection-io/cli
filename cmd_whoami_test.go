package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// With no key, a machine format is an error with nothing on standard output,
// because a script parses what is there and reads an exit 0 as an answer. The
// readable block still says so in prose and exits 0.
func TestWhoamiWithNoKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedArgs, savedFlags, savedConfig := os.Args, pflag.CommandLine, gConfig
	t.Cleanup(func() { os.Args, pflag.CommandLine, gConfig = savedArgs, savedFlags, savedConfig })

	for _, c := range []struct {
		args    []string
		wantErr bool
	}{
		{[]string{"--json"}, true},
		{[]string{"--jsonl"}, true},
		{[]string{"--format", "json"}, true},
		{nil, false},
	} {
		os.Args = append([]string{"vpndetection", "whoami"}, c.args...)
		pflag.CommandLine = pflag.NewFlagSet("vpndetection", pflag.ContinueOnError)
		gConfig = NewConfig()
		file, err := captureStdout(t, cmdWhoami)
		out, readErr := io.ReadAll(file)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if c.wantErr && (err == nil || len(out) != 0) {
			t.Errorf("whoami %v: printed %q, err %v; want an error and nothing printed", c.args, out, err)
		}
		if !c.wantErr && (err != nil || !strings.HasPrefix(string(out), "not authenticated")) {
			t.Errorf("whoami %v: printed %q, err %v; want the notice", c.args, out, err)
		}
	}
}

// A key on the free plan is on Unlimited: whoami names the plan so, and counts the day's lookups with
// every field, its daily trial, from the entitlement's usage.
func TestWhoamiOnTheFreePlan(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/entitlement" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"org_id":"f32191d0-ef02-450e-a505-eb5814c35cab",`+
			`"apikey":{"id":"10c2b437-3aa2-4a63-bd17-8e7c8c7f0def",`+
			`"expires":null,"allowed_cidrs":[]},`+
			`"plan":{"key":"free","tier":"free"},"usage":{"requests":37,"quota":100,"hard_limit":null,`+
			`"window_start":"2026-11-11T00:00:00Z","window_end":"2099-11-12T00:00:00Z"}}`)
	}))
	t.Cleanup(api.Close)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "vd_test_0123456789abcdef")
	t.Setenv("VPNDETECTION_SESSION", "")
	t.Setenv("VPNDETECTION_BASE_URL", api.URL)
	savedArgs, savedFlags, savedConfig := os.Args, pflag.CommandLine, gConfig
	t.Cleanup(func() { os.Args, pflag.CommandLine, gConfig = savedArgs, savedFlags, savedConfig })
	os.Args = []string{"vpndetection", "whoami"}
	pflag.CommandLine = pflag.NewFlagSet("vpndetection", pflag.ContinueOnError)
	gConfig = NewConfig()

	file, err := captureStdout(t, cmdWhoami)
	out, readErr := io.ReadAll(file)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, want := range []string{"plan         Unlimited\n", "every field  37 of 100 lookups today\n"} {
		if err != nil || !strings.Contains(string(out), want) {
			t.Errorf("missing %q (err %v):\n%s", want, err, out)
		}
	}
}
