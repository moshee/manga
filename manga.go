package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"ktkr.us/pkg/dn2/manga"

	"ktkr.us/pkg/manga/core"
	"ktkr.us/pkg/manga/util"
)

var (
	spreadPattern = regexp.MustCompile(`^\d+(\-|[a-zA-Z])(\d+|\w*)`)
	pagePattern   = regexp.MustCompile(`^\d+`)
	globalX       *bool
)

var commands = []*Command{
	cmdHelp,
	cmdInit,
	cmdPrep,
	cmdResize,
	cmdPkg,
	cmdUp,
	cmdNews,
	cmdTest,
	cmdConfig,
}

func main() {
	log.SetFlags(log.Lshortfile)
	log.SetPrefix("manga: ")
	if len(os.Args) < 2 {
		help(nil)
	}

	args := os.Args[1:]
	cmd := args[0]
	if len(args) > 1 {
		args = args[1:]
	} else {
		args = args[0:0]
	}

	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(os.Stderr, "\033[1;31mPANIC\033[0m: %v\n", err)
			util.PrintStack(3, 10)
		}
	}()

	for _, c := range commands {
		if c.Name == cmd && c.Run != nil {
			c.init()
			if c.Flags != nil {
				globalX = c.Flags.Bool("x", false, "Use provided file list instead")
				c.Flags.Parse(args)
				c.Run(c, c.Flags.Args())
			} else {
				c.Run(c, args)
			}
			return
		}
	}

	fmt.Fprintf(os.Stderr, "manga: no such subcommand '%s'\n", cmd)
	help(nil)
}

type Command struct {
	Run     func(cmd *Command, args []string)
	Name    string
	Summary string
	Help    string
	Flags   *flag.FlagSet
	*log.Logger
}

func (cmd *Command) init() {
	prefix := log.Prefix() + cmd.Name + ": "
	cmd.Logger = log.New(os.Stderr, prefix, 0)
}

func (cmd *Command) mkdir(name string) {
	if err := os.MkdirAll(name, 0755); err != nil {
		cmd.Fatal(err)
	}
}

// TODO: how do we identify oneshots?
func (cmd *Command) identifier(s string) core.Identifier {
	if len(s) < 2 {
		cmd.Fatalf("%s: invalid identifier", s)
	}

	n := strings.IndexFunc(s, func(ch rune) bool { return '0' <= ch && ch <= '9' })
	if n == -1 {
		cmd.Fatalf("%s: invalid identifier", s)
	}

	ord, err := strconv.Atoi(s[n:])
	if err != nil {
		cmd.Fatalf("%s: invalid identifier: %v", s, err)
	}
	id := core.Identifier{Ordinal: ord}

	switch pre := s[:n]; pre {
	case "v":
		id.Kind = manga.Volume
	case "c":
		id.Kind = manga.Chapter
	case "cd":
		id.Kind = manga.DramaCD
	default:
		cmd.Fatalf("%s: invalid kind specifier '%s' in identifier", s, pre)
	}

	return id
}

func (cmd *Command) in(id, path string) {
	cmd.identifier(id)
	if err := os.Chdir(filepath.Join(id, path)); err != nil {
		cmd.Fatal(err)
	}
}

type Link struct {
	Id        int
	ReleaseId int
	Name      string
	URL       string
}

type Progress struct {
	Id        int
	ReleaseId int
	Done      int
	Total     int
	Updated   time.Time
}

func help(cmd *Command) {
	fmt.Fprintln(os.Stderr, "manga: scanlation management and distro tool")

	if cmd == nil {
		fmt.Fprintln(os.Stderr, "Usage: manga <command> [options] [args...]")
		fmt.Fprintln(os.Stderr, "Commands:")

		tw := tabwriter.NewWriter(os.Stderr, 8, 4, 2, ' ', 0)
		for _, cmd := range commands {
			var synopsis string
			// grab the first sentence of the command description
			if i := strings.Index(cmd.Help, "."); i == -1 {
				synopsis = cmd.Help
			} else {
				synopsis = cmd.Help[:i+1]
			}

			synopsis = strings.Replace(synopsis, "\n", " ", -1)
			synopsis = strings.TrimSpace(synopsis)
			fmt.Fprintf(tw, "  %s\t%s\n", cmd.Name, synopsis)
		}
		tw.Flush()
	} else {
		fmt.Fprintf(os.Stderr, "Summary: manga %s %s\n", cmd.Name, cmd.Summary)
		fmt.Fprintln(os.Stderr, cmd.Help)

		if cmd.Flags != nil {
			fmt.Fprintln(os.Stderr)
			cmd.Flags.PrintDefaults()
		}
	}

	os.Exit(1)
}

type Error struct {
	Msg string
	Err string
}

func (e Error) Error() string {
	return e.Msg + ": " + e.Err
}
