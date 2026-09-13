package main

import (
	"os"
	"testing"
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
		{[]string{"vpndetection", "myaccount"}, "myaccount"},
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
