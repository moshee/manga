package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"ktkr.us/pkg/manga/core"
)

var cmdPrep = &Command{
	Name:    "prep",
	Summary: "[-d] [-s n₁-m₁[,n₂-m₂...]] { <identifier> | -x <files...> }",
	Help: `
Rotate and split sideways double pages from scanner. Pages such as from
doujinshi which are only one page per image need not be rotated and split up,
so use -d. If -d is not specified, the list of spreads from -spreads will be
unsplit, just rotated and named accordingly.`,
	Flags: flag.NewFlagSet("prep", flag.ExitOnError),
}

var (
	prepD = cmdPrep.Flags.Bool("d", false, "Don't rotate and crop, just rename")
	prepS = cmdPrep.Flags.String("s", "", "Skip splitting spreads named by `\033[4mLIST\033[m`")
)

func init() {
	cmdPrep.Run = runPrep
}

func runPrep(cmd *Command, args []string) {
	var ims []*Image
	if *globalX {
		ims = imageList(args[1:], ScannerPage)
	} else {
		os.Chdir(core.TopLevel())
		cmd.in(args[0], "raw")
		ims = images(ScannerPage)
	}

	sort.Sort(byScannerOrder(ims))
	// page.jpeg   (1) -> 001.jpg
	// page 1.jpeg (2) -> 002.jpg, 003.jpg

	if *prepD {
		mag := int(math.Log10(float64(len(ims)))) + 1
		for i, im := range ims {
			newName := fmt.Sprintf("%0*d.jpg", mag, i)
			newPath := filepath.Join(filepath.Dir(im.Path), newName)
			os.Rename(im.Path, newPath)
		}
	} else {
		mag := int(math.Log10(float64(len(ims)*2))) + 1

		imgdo("Prepping", ims, func(im *Image) {
			ord := im.scannerOrd()
			var first, second string
			if ord == 1 {
				// if it's the first image, that means it's missing a right hand
				// side page. But we don't know if it was scanned on the left side
				// or the right side (likely left but who knows), so we'll just
				// manually rename it later. Probably more reliable than adding a
				// flag or something.
				first = "001a.jpg"
				second = "001b.jpg"
			} else {
				n := (ord - 1) * 2
				first = fmt.Sprintf("%0*d.jpg", mag, n)
				second = fmt.Sprintf("%0*d.jpg", mag, n+1)
			}
			im.convert(
				// Do the cropping
				"-rotate", "-90", "-crop", "50%x100%",
				// save both halves into the temp register, deleting the second
				// one first (right side, first page in book order)
				"-write", "mpr:temp", "+delete",
				// write the one left in mpr:temp, the second page, then clear
				// the sequence
				"-write", second, "+delete",
				// load the sequence again, reverse it, and repeat the same
				// thing, so now the first page is the only one left
				"mpr:temp", "+swap", "+delete", first)
		})
	}

}

type byScannerOrder []*Image

func (s byScannerOrder) Len() int { return len(s) }
func (s byScannerOrder) Less(i, j int) bool {
	return s[i].scannerOrd() < s[j].scannerOrd()
}

func (s byScannerOrder) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
