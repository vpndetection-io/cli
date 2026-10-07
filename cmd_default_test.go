package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// With no address named, the null device gets the help as a terminal does: it
// is the standard input of a service, a `docker run` without -i and nohup,
// none of which handed the command anything. A pipe did, so an empty one is an
// empty input.
func TestBareRunHelpsOnTheNullDevice(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = null.Close() })
	pipe, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	t.Cleanup(func() { _ = pipe.Close() })

	out, err := bareRun(t, null)
	if err != nil || !strings.HasPrefix(out, "Usage: ") {
		t.Errorf("stdin from %s: printed %q, err %v; want the help", os.DevNull, out, err)
	}
	out, err = bareRun(t, pipe)
	if err == nil || !strings.Contains(err.Error(), "no addresses found in the input") {
		t.Errorf("stdin from an empty pipe: printed %q, err %v; want the empty-input error", out, err)
	}
}

// bareRun runs the default command with no arguments and stdin from a file, and
// returns what it printed.
func bareRun(t *testing.T, stdin *os.File) (string, error) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedArgs, savedFlags, savedStdin, savedConfig := os.Args, pflag.CommandLine, os.Stdin, gConfig
	t.Cleanup(func() {
		os.Args, pflag.CommandLine, os.Stdin, gConfig = savedArgs, savedFlags, savedStdin, savedConfig
	})
	os.Args = []string{"vpndetection"}
	pflag.CommandLine = pflag.NewFlagSet("vpndetection", pflag.ContinueOnError)
	os.Stdin = stdin
	gConfig = NewConfig()
	gConfig.CacheEnabled = false

	out, err := captureStdout(t, cmdDefault)
	printed, readErr := io.ReadAll(out)
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(printed), err
}
