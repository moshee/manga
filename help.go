package main

var cmdHelp = &Command{
	Name:    "help",
	Summary: "[command]",
	Help:    "Show help for the specified command.",
}

func init() {
	cmdHelp.Run = runHelp
}

func runHelp(cmd *Command, args []string) {
	if len(args) == 0 {
		help(nil)
	}

	for _, c := range commands {
		if c.Name == args[0] {
			help(c)
			return
		}
	}

	cmd.Fatalf("no such command: %s", args[0])
}
