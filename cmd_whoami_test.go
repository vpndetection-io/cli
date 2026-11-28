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

// whoami names the plan as the API serves it, never from its key, which a plan keeps however it is
// renamed: a renamed free plan keeps its daily lines, and a paid plan shows its name and tier.
func TestWhoamiNamesThePlanAsServed(t *testing.T) {
	for _, c := range []struct {
		plan    string
		want    []string
		notWant string
	}{
		{
			`{"key":"free","name":"Free","tier":"free"}`,
			[]string{"plan         Free\n", "every field  37 of 100 lookups today\n"},
			"\nfields ",
		},
		{
			`{"key":"free","name":"Open","tier":"free"}`,
			[]string{"plan         Open\n", "every field  37 of 100 lookups today\n"},
			"Free",
		},
		{
			`{"key":"data","name":"Data Pro","tier":"max"}`,
			[]string{"plan         Data Pro\n", "fields       max tier\n", "used         37 of 100\n"},
			"plan         data",
		},
	} {
		out := whoamiAgainst(t, `{"org_id":"f32191d0-ef02-450e-a505-eb5814c35cab",`+
			`"apikey":{"id":"10c2b437-3aa2-4a63-bd17-8e7c8c7f0def",`+
			`"expires":null,"allowed_cidrs":[]},`+
			`"plan":`+c.plan+`,"usage":{"requests":37,"quota":100,"hard_limit":null,`+
			`"window_start":"2026-11-11T00:00:00Z","window_end":"2099-11-12T00:00:00Z"}}`)
		for _, want := range c.want {
			if !strings.Contains(out, want) {
				t.Errorf("plan %s: missing %q:\n%s", c.plan, want, out)
			}
		}
		if strings.Contains(out, c.notWant) {
			t.Errorf("plan %s: printed %q:\n%s", c.plan, c.notWant, out)
		}
	}
}

// A free key's lookups with every field are a day's, its usage window (2026-11-28): whoami says
// what a lookup past them gets and when they reset, spent or not, and never names a trial.
func TestWhoamiSaysWhenAFreeKeysDayResets(t *testing.T) {
	for _, requests := range []string{"37", "100"} {
		out := whoamiAgainst(t, `{"org_id":"f32191d0-ef02-450e-a505-eb5814c35cab",`+
			`"apikey":{"id":"10c2b437-3aa2-4a63-bd17-8e7c8c7f0def","expires":null,"allowed_cidrs":[]},`+
			`"plan":{"key":"free","name":"Free","tier":"free"},"usage":{"requests":`+requests+
			`,"quota":100,"hard_limit":null,"window_start":"2099-11-11T00:00:00Z",`+
			`"window_end":"2099-11-12T00:00:00Z"}}`)
		want := "\npast it      lookups answer with the basics, no VPN flag, until they reset\n" +
			"resets       2099-11-12T00:00:00Z (in "
		if !strings.Contains(out, want) || strings.Contains(out, "trial") {
			t.Errorf("%s of 100: want %q, and no trial:\n%s", requests, want, out)
		}
	}
}

// A paid key's hard limit, its quota while overage is off or the overage cap, says what a lookup
// past it gets; with no limit, requests past the quota are billed as overage.
func TestWhoamiSaysWhatHappensPastTheHardLimit(t *testing.T) {
	for _, c := range []struct{ limit, want, notWant string }{
		{"100", "hard limit   100\npast it      lookups answer with the basics and VPN flag, unbilled, " +
			"until usage resets\n", "billed as overage"},
		{"null", "hard limit   none; requests above the quota are billed as overage\n", "past it"},
	} {
		out := whoamiAgainst(t, `{"org_id":"f32191d0-ef02-450e-a505-eb5814c35cab",`+
			`"apikey":{"id":"10c2b437-3aa2-4a63-bd17-8e7c8c7f0def","expires":null,"allowed_cidrs":[]},`+
			`"plan":{"key":"max","name":"Max","tier":"max"},"usage":{"requests":37,"quota":100,`+
			`"hard_limit":`+c.limit+`,"window_start":"2026-11-11T00:00:00Z","window_end":"2099-11-12T00:00:00Z"}}`)
		if !strings.Contains(out, c.want) || strings.Contains(out, c.notWant) {
			t.Errorf("hard_limit %s: want %q and no %q:\n%s", c.limit, c.want, c.notWant, out)
		}
	}
}

// whoamiAgainst runs the readable whoami against an API serving answer as the entitlement.
func whoamiAgainst(t *testing.T, answer string) string {
	t.Helper()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/entitlement" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, answer)
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
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
