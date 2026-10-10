package main

import (
	"io"
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
