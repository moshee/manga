package main

import (
	"flag"
	"path/filepath"
)

var cmdInit = &Command{
	Name:    "init",
	Summary: "[-n] <identifier>",
	Help: `
Initializes a new work directory in the current series.`,
	Flags: flag.NewFlagSet("init", flag.ExitOnError),
}

var (
	initN = cmdInit.Flags.Bool("n", false, "Create a whole new series")
)

func init() {
	cmdInit.Run = runInit
}

func runInit(cmd *Command, args []string) {
	if len(args) == 0 {
		help(cmd)
	}

	cmd.identifier(args[0])

	for _, d := range []string{"raw", "res", "psd"} {
		cmd.mkdir(filepath.Join(args[0], d))
	}
}
