package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"ktkr.us/pkg/manga/util"
)

type ImageKind int

const (
	Unknown ImageKind = iota
	Page
	Spread
	ScannerPage
)

type Rect struct{ W, H int }

type Image struct {
	Path string
	Kind ImageKind

	// invalid until call to size()
	Rect
}

func namedImage(name string) *Image {
	fi, err := os.Stat(name)
	if err != nil {
		log.Fatalf("image: %v", err)
	}
	if fi.IsDir() {
		log.Fatalf("image: %s is a directory", name)
	}
	return newImage(fi)
}

func imageSizes(ims []*Image) {
	if len(ims) == 0 {
		return
	}
	args := make([]string, 0, len(ims)+2)
	args = append(args, "-format", "%w %h\n")
	for _, im := range ims {
		args = append(args, im.Path)
	}

	out, _ := util.System("identify", args...)
	sizes := strings.Split(out, "\n")

	for i, im := range ims {
		pair := strings.Split(strings.TrimSpace(sizes[i]), " ")
		if len(pair) < 2 {
			log.Fatal("imageSizes: malformed `identify` response: ", sizes[i])
		}
		im.W, _ = strconv.Atoi(pair[0])
		im.H, _ = strconv.Atoi(pair[1])
	}
}

func newImage(fi os.FileInfo) *Image {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("images: %v", err)
	}

	var (
		filename = fi.Name()
		kind     = Unknown
		ext      = filepath.Ext(filename)
		name     = filepath.Base(filename)[:len(filename)-len(ext)]
	)

	switch ext {
	case ".jpeg":
		kind = ScannerPage
	case ".png", ".jpg":
		if spreadPattern.MatchString(name) {
			kind = Spread
		} else if pagePattern.MatchString(name) {
			kind = Page
		}
	}

	return &Image{
		Path: filepath.Join(wd, filename),
		Kind: kind,
	}
}

func images(kinds ...ImageKind) []*Image {
	fis, err := ioutil.ReadDir(".")
	if err != nil {
		log.Fatalf("images: %v", err)
	}

	ims := make([]*Image, 0, len(fis))

	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}
		im := newImage(fi)
		if im.Kind != Unknown {
			ims = append(ims, im)
		}
	}

	return filterImages(ims, kinds...)
}

func imageList(names []string, kinds ...ImageKind) []*Image {
	fis := make([]os.FileInfo, len(names))
	for i, name := range names {
		fi, err := os.Stat(name)
		if err != nil {
			log.Fatalf("images: %v", err)
		}
		fis[i] = fi
	}

	ims := make([]*Image, 0, len(fis))

	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}
		im := newImage(fi)
		if im.Kind != Unknown {
			ims = append(ims, im)
		}
	}

	return filterImages(ims, kinds...)
}

func filterImages(in []*Image, kinds ...ImageKind) []*Image {
	if len(kinds) == 0 {
		return []*Image{}
	}

	out := make([]*Image, 0, len(in))

outer:
	for _, im := range in {
		if im.Kind == Unknown {
			continue
		}

		for _, kind := range kinds {
			if im.Kind == kind {
				out = append(out, im)
				continue outer
			}
		}
	}

	return out
}

func (im *Image) ext() string {
	return filepath.Ext(im.Path)
}

func (im *Image) base() string {
	return filepath.Base(im.Path)
}

func (im *Image) name() string {
	return strings.TrimSuffix(im.base(), im.ext())
}

func (im *Image) convert(args ...string) {
	//args = append([]string{"convert", im.Path}, args...)
	//util.System("gm", args...)
	args = append([]string{im.Path}, args...)
	util.System("convert", args...)
}

func (im *Image) ord() int {
	s := strings.Split(im.name(), "-")
	n, _ := strconv.Atoi(s[0])
	return n
}

func (im *Image) scannerOrd() int {
	name := strings.TrimFunc(im.name(), func(r rune) bool {
		return !unicode.IsDigit(r)
	})
	if len(name) == 0 {
		return 1
	}
	n, _ := strconv.Atoi(name)
	return n + 1
}

func (im *Image) size() (w, h int) {
	out, _ := util.System("identify", "-format", "%w %h", im.Path)
	pair := strings.Split(out, " ")
	im.W, _ = strconv.Atoi(pair[0])
	im.H, _ = strconv.Atoi(pair[1])
	return im.W, im.H
}

func (im *Image) open() *os.File {
	file, err := os.Open(im.Path)
	if err != nil {
		log.Fatal(err)
	}
	return file
}

func (im *Image) optimize() {
	switch im.ext() {
	case ".jpg":
		util.System("jpegoptim", "--strip-all", im.Path)
	case ".png":
		util.System("optipng", "-quiet", im.Path)
		//util.System("pngout", im.Path, "/c0")
		// why the fuck is pngout returning status 2 without printing anything?
	}
}

// performs f on each image in parallel with GOMAXPROCS workers
func imgdo(banner string, ims []*Image, f func(*Image)) {
	switch len(ims) {
	case 1:
		f(ims[0])
		fallthrough
	case 0:
		return
	}

	var (
		procs     = runtime.GOMAXPROCS(-1)
		work      = make(chan *Image)
		punchcard = make(chan int)
		done      = make(chan bool)
	)

	if len(ims) < procs {
		procs = len(ims)
	}

	for i := 0; i < procs; i++ {
		go func(work <-chan *Image, punchcard chan<- int, workerId int) {
			for im := range work {
				punchcard <- workerId
				f(im)
			}
			done <- true
		}(work, punchcard, i)
	}

	fmt.Printf("Using %d worker%s\n", procs, util.Plural(procs))
	workers := make([]string, procs)
	for i := range workers {
		workers[i] = "-"
	}
	mag := strconv.Itoa(int(math.Log10(float64(len(ims)))) + 1)
	format := "\033[J%s (%" + mag + "d/%d): %s\033[1A\n"

	for i, im := range ims {
		work <- im
		workers[<-punchcard] = im.base()
		status := strings.Join(workers, " / ")
		fmt.Printf(format, banner, i+1, len(ims), status)
	}

	close(work)
	for i := 0; i < procs; i++ {
		<-done
	}
	fmt.Println()
}
