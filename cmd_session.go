package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

func printHelpSession() {
	fmt.Printf(
		`Usage: %[1]s session [list | use <name> | show [<name>] | rename <old> <new> | rm <name>]

Description:
  Named credentials. Each holds an API key and, optionally, the API it talks to,
  so one machine can hold several organizations' keys, or staging alongside
  production, and switch between them without logging in again.

  Create one with '%[1]s login --session <name>'.

Examples:
  $ %[1]s session                    # same as 'session list'
  $ %[1]s session use work
  $ %[1]s session show staging
  $ %[1]s session rename work acme
  $ %[1]s session rm old

Options:
  --help, -h
    show help.
`, progBase)
}

func cmdSession() error {
	globalFlags()
	args := parseSubFlags()

	if fHelp {
		printHelpSession()
		return nil
	}
	if len(args) == 0 {
		return sessionList()
	}

	switch strings.ToLower(args[0]) {
	case "list", "ls":
		return sessionList()
	case "use", "switch":
		if len(args) != 2 {
			return errors.New("usage: session use <name>")
		}
		return sessionUse(args[1])
	case "show":
		name := gConfig.ActiveSessionName()
		if len(args) == 2 {
			name = args[1]
		}
		return sessionShow(name)
	case "rename", "mv":
		if len(args) != 3 {
			return errors.New("usage: session rename <old> <new>")
		}
		return sessionRename(args[1], args[2])
	case "rm", "remove", "delete":
		if len(args) != 2 {
			return errors.New("usage: session rm <name>")
		}
		return sessionRemove(args[1])
	default:
		printHelpSession()
		return fmt.Errorf("%q is not a session subcommand", args[0])
	}
}

func sessionList() error {
	names := gConfig.SessionNames()
	if len(names) == 0 {
		fmt.Printf("no sessions; run `%s login`\n", progBase)
		return nil
	}
	active := gConfig.ActiveSessionName()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\tNAME\tKEY\tAPI\tLAST USED")
	for _, name := range names {
		s := gConfig.Sessions[name]
		marker := " "
		if name == active {
			marker = "*"
		}
		api := s.BaseURL
		if api == "" {
			api = "default"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", marker, name, maskKey(s.Key), api, since(s.LastUsed))
	}
	return w.Flush()
}

func sessionUse(name string) error {
	if gConfig.Sessions[name] == nil {
		return fmt.Errorf("no session named %q; `%s session list` shows what there is", name, progBase)
	}
	gConfig.Active = name
	if err := SaveConfig(gConfig); err != nil {
		return err
	}
	fmt.Printf("session %q is now active\n", name)
	return nil
}

func sessionShow(name string) error {
	if name == "" {
		fmt.Printf("no active session; run `%s login`\n", progBase)
		return nil
	}
	s := gConfig.Sessions[name]
	if s == nil {
		return fmt.Errorf("no session named %q", name)
	}
	api := s.BaseURL
	if api == "" {
		api = "default (https://api.vpndetection.io)"
	}
	fmt.Printf("name        %s\n", name)
	fmt.Printf("key         %s\n", maskKey(s.Key))
	fmt.Printf("fingerprint %s\n", keyFingerprint(s.Key))
	fmt.Printf("api         %s\n", api)
	fmt.Printf("created     %s\n", s.Created.Format(time.RFC3339))
	fmt.Printf("last used   %s\n", since(s.LastUsed))
	return nil
}

func sessionRename(oldName, newName string) error {
	s := gConfig.Sessions[oldName]
	if s == nil {
		return fmt.Errorf("no session named %q", oldName)
	}
	if newName == "" {
		return errors.New("the new name cannot be empty")
	}
	if gConfig.Sessions[newName] != nil {
		return fmt.Errorf("a session named %q already exists", newName)
	}
	delete(gConfig.Sessions, oldName)
	gConfig.Sessions[newName] = s
	if gConfig.Active == oldName {
		gConfig.Active = newName
	}
	if err := SaveConfig(gConfig); err != nil {
		return err
	}
	fmt.Printf("renamed %q to %q\n", oldName, newName)
	return nil
}

func sessionRemove(name string) error {
	if gConfig.Sessions[name] == nil {
		return fmt.Errorf("no session named %q", name)
	}
	delete(gConfig.Sessions, name)
	if gConfig.Active == name {
		gConfig.Active = ""
		if names := gConfig.SessionNames(); len(names) > 0 {
			gConfig.Active = names[0]
		}
	}
	if err := SaveConfig(gConfig); err != nil {
		return err
	}
	fmt.Printf("removed session %q\n", name)
	if gConfig.Active != "" {
		fmt.Printf("session %q is now active\n", gConfig.Active)
	}
	return nil
}

// since renders a timestamp as a rough age, or "never" for the zero value.
func since(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
