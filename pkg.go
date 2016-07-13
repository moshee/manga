package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ktkr.us/pkg/dn2/manga"

	"ktkr.us/pkg/manga/core"
	"ktkr.us/pkg/manga/job"
	"ktkr.us/pkg/manga/util"
)

var cmdPkg = &Command{
	Name:    "pkg",
	Summary: "[-O] [-n] [-f] [-c] <identifier> [extra...]",
	Help: `
Package pages into zip files.`,
	Flags: flag.NewFlagSet("pkg", flag.ExitOnError),
}

var (
	pkgO = cmdPkg.Flags.Bool("O", false, "Optimize images before packaging")
	pkgN = cmdPkg.Flags.Bool("n", false, "Skip splitting into individual chapter zips")
	pkgF = cmdPkg.Flags.Bool("f", false, "Skip archives that already exist")
	pkgC = cmdPkg.Flags.Bool("c", false, "Only package chapters, skip volume")
)

func init() {
	cmdPkg.Run = runPkg
}

func runPkg(cmd *Command, args []string) {
	core.LoadConfig()

	if len(args) == 0 {
		help(cmd)
	}

	id := cmd.identifier(args[0])

	// build zip file name
	wd, err := os.Getwd()
	if err != nil {
		cmd.Fatal(err)
	}

	zipPath := filepath.Join(wd, makeZipName(id, args))

	cmd.in(id.String(), "res")
	ims := images(Page, Spread)
	zips := []*ZipDest{}
	if !*pkgC {
		zips = append(zips, &ZipDest{zipPath, ims, false})
	}
	os.Chdir(core.TopLevel())

	if *pkgO {
		imgdo("Optimizing", ims, (*Image).optimize)
	}

	if !*pkgN && id.Kind == manga.Volume {
		chaps := core.ParseSplits(id)
		doSplits(cmd, chaps, &zips, ims, wd)
	}

	// check to see if any of the zip file names exist already
	for _, zipDest := range zips {
		if _, err = os.Stat(zipDest.Name); err == nil {
			zipName := filepath.Base(zipDest.Name)
			if *pkgF {
				cmd.Printf("will overwrite \033[4m%s\033[0m", zipName)
			} else {
				// TODO: check all of the images' modified times to see if we
				// should make a new archive
				cmd.Printf("file \033[4m%s\033[0m already exists", zipName)
				cmd.Print("  (use flag -f to ignore)")
				os.Exit(1)
			}
		}
	}

	t := make(job.Group, len(zips))
	for i, zd := range zips {
		t[i] = job.New(zd, filepath.Base(zd.Name))
	}

	if err := t.Begin(); err != nil {
		cmdPkg.Fatal(err)
	}
}

func makeZipName(id core.Identifier, args []string) string {
	parts := []string{core.Config.Title, id.String()}
	if args != nil && len(args) > 1 {
		parts = append(parts, args[1:]...)
	}
	parts = append(parts, "["+core.Config.Group+"].zip")

	return strings.Join(parts, " ")
}

func doSplits(cmd *Command, chaps []*core.ChapSplit, zips *[]*ZipDest, ims []*Image, wd string) {
	i, j := 0, 0
split:
	for n, chap := range chaps {
		// fields = <starting page> <chapter #> [chapter title]
		for j = i; j < len(ims); j++ {
			if ims[j].name() == chap.Name {
				// skip empty ranges & also the first range with nonexistent
				// lower end
				if n > 0 {
					chap = chaps[n-1]
				}
				if i != j {
					*zips = append(*zips, &ZipDest{
						util.Rooted(chap.ZipName()),
						ims[i:j],
						false,
					})
				}
				i = j
				continue split
			}
		}

		cmdPkg.Fatalf("Splitfile: specified page doesn't exist (%v)", chap)
	}

	chap := chaps[len(chaps)-1]
	*zips = append(*zips, &ZipDest{
		util.Rooted(chap.ZipName()),
		ims[i:],
		false,
	})
}

type ZipDest struct {
	Name   string
	Images []*Image
	Skip   bool
}

func (z *ZipDest) String() string {
	return fmt.Sprintf("%s (%d images) (skip = %v)", z.Name, len(z.Images), z.Skip)
}

func (zd *ZipDest) Begin(p chan string) error {
	file, err := os.OpenFile(zd.Name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	z := zip.NewWriter(file)
	defer z.Close()

	for i, im := range zd.Images {
		imFile := im.open()
		defer imFile.Close()
		fi, err := imFile.Stat()
		if err != nil {
			return err
		}
		fih, err := zip.FileInfoHeader(fi)
		if err != nil {
			return err
		}

		fih.Method = zip.Deflate

		w, err := z.CreateHeader(fih)
		if err != nil {
			return err
		}

		io.Copy(w, imFile)
		p <- fmt.Sprintf("%d / %d", i+1, len(zd.Images))
	}

	return nil
}
func filesizes(names ...string) (totalSize util.Bytes) {
	for _, name := range names {
		fi, err := os.Stat(name)
		if err != nil {
			cmdPkg.Fatal(err)
		}

		totalSize += util.Bytes(fi.Size())
	}

	return
}
