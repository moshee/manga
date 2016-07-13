package util

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"ktkr.us/pkg/manga/core"
)

// terminal

type Direction string

const (
	Up     Direction = "A"
	Down   Direction = "B"
	Left   Direction = "C"
	Right  Direction = "D"
	Column Direction = "G"
)

func Cursor(dir Direction, num int) {
	fmt.Fprintf(os.Stderr, "\033[%d%s", num, dir)
}

/*
 * Stat writer
 */

type writeRet struct {
	n   int
	err error
}

// StatWriter is a writer that reports its progress to a channel.
type StatWriter struct {
	w        io.Writer
	buf      []Bytes
	ptr      int
	write    chan []byte
	ret      chan writeRet
	interval time.Duration
	t        *time.Ticker
	progress Bytes
	total    Bytes
	ch       chan<- string
}

// newStatWriter constructs a new StatWriter from a given source
func NewStatWriter(w io.Writer, bufsize int, interval time.Duration, totalsize Bytes, ch chan<- string) *StatWriter {
	return &StatWriter{
		w:        w,
		buf:      make([]Bytes, bufsize),
		write:    make(chan []byte),
		ret:      make(chan writeRet),
		interval: interval,
		t:        time.NewTicker(interval),
		total:    totalsize,
		ch:       ch,
	}
}

func (sw *StatWriter) Write(p []byte) (n int, err error) {
	sw.write <- p
	q := <-sw.ret
	return q.n, q.err
}

// Report will start logging writes and sending the statistics to sw.ch every
// sw.interval. If an error is recieved on ch, the write will terminate and the
// error will be returned.
func (sw *StatWriter) Report(ch <-chan error) error {
	var err error
loop:
	for {
		select {
		case err = <-ch:
			break loop

		case p := <-sw.write:
			n, err := sw.w.Write(p)
			sw.ret <- writeRet{n, err}
			sw.buf[sw.ptr] += Bytes(n)
			if err != nil {
				break loop
			}
			sw.progress += Bytes(n)

		case <-sw.t.C:
			if sw.total > Bytes(0) {
				b := Bytes(0)
				for _, i := range sw.buf {
					b += i
				}
				b /= Bytes(len(sw.buf))

				speed := float64(b) / (float64(sw.interval) / float64(time.Second))
				status := fmt.Sprintf("%v/%v @ %v/s ", sw.progress, sw.total, Bytes(speed))

				pct := 100.0 * (float64(sw.progress) / float64(sw.total))
				if pct >= 99.0 {
					status += "(almost there!)"
				} else {
					status += fmt.Sprintf("(%.0f%%)", pct)
				}

				sw.ch <- status

				if len(sw.buf) < cap(sw.buf) {
					sw.buf = append(sw.buf, 0)
				}
				sw.ptr = (sw.ptr + 1) % len(sw.buf)
				sw.buf[sw.ptr] = 0
			}
		}
	}
	if sw.total > Bytes(0) && sw.progress >= sw.total {
		sw.ch <- fmt.Sprintf("%v/%v - done", sw.total, sw.total)
	}
	return err
}

type Bytes int64

const (
	B Bytes = 1 << (10 * iota)
	KiB
	MiB
	GiB
	TiB
)

func (b Bytes) String() string {
	switch {
	case b < KiB:
		return fmt.Sprintf("%d B", b)
	case b < MiB:
		return fmt.Sprintf("%.2f KiB", float64(b)/float64(KiB))
	case b < GiB:
		return fmt.Sprintf("%.2f MiB", float64(b)/float64(MiB))
	case b < TiB:
		return fmt.Sprintf("%.2f GiB", float64(b)/float64(GiB))
	default:
		return fmt.Sprintf("%.2f TiB", float64(b)/float64(TiB))
	}
}

func System(name string, args ...string) (out, err string) {
	cmd := exec.Command(name, args...)
	buf := new(bytes.Buffer)
	cmd.Stderr = buf
	cmd.Stdout = buf

	if err := cmd.Run(); err != nil {
		fmt.Fprint(os.Stderr, buf.String())
		log.Fatalf("exec %s: %v", name, err)
	}

	str := buf.String()
	return str, str
}

func Launch(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Fatalf("exec %s: %v", name, err)
	}

	if err := cmd.Wait(); err != nil {
		log.Fatalf("exec %s: %v", name, err)
	}
}

func GetEditor() string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	return editor
}

func Promptf(s string, args ...interface{}) bool {
	fmt.Printf(s+" ", args...)
	for {
		in := ""
		fmt.Scanln(&in)
		in = strings.ToLower(in)
		switch in {
		case "y":
			return true
		case "n":
			return false
		default:
			fmt.Fprint(os.Stderr, "\033[1;4my\033[0mes or \033[1;4mn\033[0mo? ")
		}
	}
}

func PrintStack(skip, count int) {
	pcs := make([]uintptr, count)
	s := runtime.Callers(skip, pcs)
	pcs = pcs[:s]
	tw := tabwriter.NewWriter(os.Stderr, 4, 8, 1, ' ', 0)

	for i, pc := range pcs {
		f := runtime.FuncForPC(pc)
		path, line := f.FileLine(pc)
		name := f.Name()
		parent, file := filepath.Split(path)
		parent, oneUp := filepath.Split(filepath.Clean(parent))
		file = filepath.Join(oneUp, file)

		fmt.Fprintf(tw, "%2d  %s:%d\t@ 0x%x\t%s\n", i, file, line, pc, name)
	}
	tw.Flush()
}

func Plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func Rooted(in ...string) string {
	wd, _ := os.Getwd()
	s := filepath.Join(append([]string{core.TopLevel()}, in...)...)
	os.Chdir(wd)
	return s
}
