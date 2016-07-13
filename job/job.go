package job

import (
	"fmt"
	"os"

	"ktkr.us/pkg/manga/util"
)

type Job interface {
	Begin(chan string) error
}

type Func func(chan string) error

func (f Func) Begin(progress chan string) error { return f(progress) }

type Running struct {
	id       int
	name     string
	job      Job
	progress chan string // Send strings here to be printed out.
	err      error
}

func New(j Job, name string) *Running {
	return &Running{job: j, name: name}
}

type progress struct {
	j *Running
	p string
}

type Group []*Running

// Begin runs a job group and waits until all the jobs finish. If any one of
// them returns an error, the whole group will be canelled and that error will
// be returned.
func (g Group) Begin() error {
	// Figure out how to visually align the printout.
	nameSize := 0
	for _, j := range g {
		if l := len(j.name); l > nameSize {
			nameSize = l
		}
	}
	format := fmt.Sprintf("\033[K%%-%ds   %%s", nameSize)

	done := make(chan *Running)
	prg := make(chan progress)

	// Fan-out jobs.
	for i := range g {
		fmt.Fprintln(os.Stderr)
		go func(i int) {
			j := g[i]
			j.id = i
			j.progress = make(chan string, 5)
			prg <- progress{j, "Waiting..."}

			// Start the job.
			go func(j *Running) {
				j.err = j.job.Begin(j.progress)
				close(j.progress)
			}(j)

			// Drain the job's progress channel and then exit.
			for p := range j.progress {
				prg <- progress{j, p}
			}
			done <- j
		}(i)
	}

	// Fan-in job progress and update lines in print-out accordingly. We only
	// want one goroutine printing stuff out.
	a := len(g)
	for {
		select {
		case p := <-prg:
			g.updateLine(p, format)
		case j := <-done:
			if j.err != nil {
				g.updateLine(progress{j, "Error"}, format)
				return j.err
			}
			g.updateLine(progress{j, "Done"}, format)
			a--
			if a <= 0 {
				return nil
			}
		}
	}
}

func (g Group) updateLine(p progress, format string) {
	shift := len(g) - p.j.id
	util.Cursor(util.Column, 0)
	util.Cursor(util.Up, shift)
	fmt.Fprintf(os.Stderr, format, p.j.name, p.p)
	util.Cursor(util.Column, 0)
	util.Cursor(util.Down, shift)
}
