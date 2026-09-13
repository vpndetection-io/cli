package main

import (
	"fmt"
	"strings"

	"github.com/mslmio/libgo-complete/install"
)

func printHelpCompletion() {
	fmt.Printf(
		`Usage: %[1]s completion [install | uninstall | bash | zsh | fish]

Description:
  Shell auto-completion for commands, flags, and the values flags take.

  'install' writes into every shell it finds; start a new shell afterwards. The
  per-shell subcommands print the line instead, for adding by hand.

Examples:
  $ %[1]s completion install
  $ %[1]s completion bash >> ~/.bashrc

Options:
  --help, -h
    show help.
`, progBase)
}

func cmdCompletion() error {
	globalFlags()
	args := parseSubFlags()

	if fHelp || len(args) != 1 {
		printHelpCompletion()
		return nil
	}

	var line string
	var err error
	switch strings.ToLower(args[0]) {
	case "install":
		if err := install.Install(progBase); err != nil {
			return err
		}
		fmt.Println("installed; start a new shell to pick it up")
		return nil
	case "uninstall":
		if err := install.Uninstall(progBase); err != nil {
			return err
		}
		fmt.Println("uninstalled")
		return nil
	case "bash":
		line, err = install.BashCmd(progBase)
	case "zsh":
		line, err = install.ZshCmd(progBase)
	case "fish":
		line, err = install.FishCmd(progBase)
	default:
		printHelpCompletion()
		return fmt.Errorf("%q is not a shell this knows", args[0])
	}
	if err != nil {
		return err
	}
	fmt.Println(line)
	return nil
}
