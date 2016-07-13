package core

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type ChapSplit struct {
	Id    Identifier
	Name  string // page range
	Num   string
	Title string
}

func (s *ChapSplit) ZipName() string {
	return s.String() + ".zip"
}

func (s *ChapSplit) String() string {
	if s.Title == "" {
		return fmt.Sprintf("%s %s", s.Id, s.Num)
	}
	return fmt.Sprintf("%s c%s - %s", s.Id, s.Num, s.Title)
}

// ParseSplits loads and parses the Splitfile for the associated volume
// (assuming id is a volume)
func ParseSplits(id Identifier) []*ChapSplit {
	// check for Splitfile
	splitpath := filepath.Join(TopLevel(), id.String(), "Splitfile")
	splitfile, err := os.Open(splitpath)
	if err != nil {
		log.Fatal(err)
	}
	defer splitfile.Close()

	s := bufio.NewScanner(splitfile)
	chaps := make([]*ChapSplit, 0)

	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 2 {
			log.Fatal("Malformed Splitfile")
		}
		chap := &ChapSplit{Id: id, Name: fields[0], Num: fields[1]}
		if len(fields) > 2 {
			chap.Title = strings.Trim(strings.Join(fields[2:], " "), ".")
		}
		chaps = append(chaps, chap)
	}

	return chaps
}
