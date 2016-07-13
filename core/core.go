package core

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"ktkr.us/pkg/dn2/manga"
)

type Identifier struct {
	Kind    manga.ReleaseKind
	Ordinal int
}

func (id Identifier) String() string {
	return fmt.Sprintf("%s%02d", id.Kind, id.Ordinal)
}

var ROOT string

// Get the top level of the manga directory
func TopLevel() string {
	if ROOT != "" {
		return ROOT
	}
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	for {
		_, err := os.Stat(filepath.Join(wd, ".manga"))
		if err == nil {
			ROOT = wd
			return wd
		}
		newwd := filepath.Dir(wd)
		if newwd == wd {
			log.Fatal("not in manga project (or any parent directories) - missing .manga")
		}
		wd = newwd
	}
}

func FirstArchive(id Identifier) (os.FileInfo, error) {
	fis, err := ioutil.ReadDir(TopLevel())
	if err != nil {
		return nil, err
	}

	zipName := fmt.Sprintf("%s %s", Config.Title, id)

	for _, fi := range fis {
		name := fi.Name()
		if strings.HasPrefix(name, zipName) && filepath.Ext(name) == ".zip" {
			return fi, nil
		}
	}

	return nil, fmt.Errorf("no zip archive found starting with '%s'", zipName)
}
