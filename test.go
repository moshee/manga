package main

import "flag"

var cmdTest = &Command{
	Name:    "test",
	Summary: "",
	Help:    "this is a test command",
	Flags:   flag.NewFlagSet("test", flag.ExitOnError),
}

func init() {
	cmdTest.Run = runTest
}

func runTest(cmd *Command, args []string) {
}
