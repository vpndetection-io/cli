package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// A flag may come before the subcommand, and a flag VALUE that happens to spell
// a subcommand must not be taken for one.
func TestDetectCommand(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"vpndetection"}, ""},
		{[]string{"vpndetection", "1.1.1.1"}, ""},
		{[]string{"vpndetection", "1.1.1.1", "2.2.2.2"}, ""},
		{[]string{"vpndetection", "db", "list"}, "db"},
		{[]string{"vpndetection", "database", "list"}, "database"},

		// A flag first.
		{[]string{"vpndetection", "--nocache", "db", "list"}, "db"},
		{[]string{"vpndetection", "--session", "work", "db", "list"}, "db"},
		{[]string{"vpndetection", "-k", "secret", "whoami"}, "whoami"},
		{[]string{"vpndetection", "--session=work", "db"}, "db"},

		// The value of a value-taking flag is never a command, however it is
		// spelled.
		{[]string{"vpndetection", "--session", "database"}, ""},
		{[]string{"vpndetection", "--session", "version", "1.1.1.1"}, ""},
		{[]string{"vpndetection", "-f", "version", "1.1.1.1"}, ""},

		// An address cannot be followed by a command; everything after it is
		// more input.
		{[]string{"vpndetection", "1.1.1.1", "db"}, ""},

		// Aliases.
		{[]string{"vpndetection", "entitlement"}, "entitlement"},
		{[]string{"vpndetection", "vsn"}, "vsn"},
		{[]string{"vpndetection", "v"}, "v"},
	}
	for _, c := range cases {
		saved := os.Args
		os.Args = c.args
		got := detectCommand()
		os.Args = saved
		if got != c.want {
			t.Errorf("detectCommand(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

// A bare command group prints its help, as a bare invocation does, rather than
// running whichever of its subcommands would be most useful.
func TestBareGroupPrintsHelp(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("VPNDETECTION_API_KEY", "")
	t.Setenv("VPNDETECTION_SESSION", "")
	savedArgs, savedFlags, savedConfig := os.Args, pflag.CommandLine, gConfig
	t.Cleanup(func() { os.Args, pflag.CommandLine, gConfig = savedArgs, savedFlags, savedConfig })

	cases := []struct {
		name string
		cmd  func() error
	}{
		{"session", cmdSession},
		{"sessions", cmdSession},
		{"database", cmdDatabase},
		{"db", cmdDatabase},
		{"cache", cmdCache},
		{"config", cmdConfig},
		{"completion", cmdCompletion},
	}
	for _, c := range cases {
		os.Args = []string{"vpndetection", c.name}
		pflag.CommandLine = pflag.NewFlagSet("vpndetection", pflag.ContinueOnError)
		gConfig = NewConfig()
		file, err := captureStdout(t, c.cmd)
		out, readErr := io.ReadAll(file)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if err != nil || !strings.HasPrefix(string(out), "Usage: ") {
			t.Errorf("bare %s: printed %q, err %v; want its help", c.name, out, err)
		}
	}
}
