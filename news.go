package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ktkr.us/pkg/dn2/manga"

	"ktkr.us/pkg/manga/core"
	"ktkr.us/pkg/manga/util"
)

var cmdNews = &Command{
	Name:    "news",
	Summary: "[-u | <title> ]",
	Help: `
Post or update news.`,
	Flags: flag.NewFlagSet("news", flag.ExitOnError),
}

func init() {
	cmdNews.Run = doNews
}

var (
	newsU = cmdNews.Flags.Bool("u", false, "Update last post instead of creating")
)

func doNews(cmd *Command, args []string) {
	tmpName := "manga-newspost_" + time.Now().Format("2006-01-02_15_04_05")
	tmpPath := filepath.Join(os.TempDir(), tmpName)
	file, err := os.Create(tmpPath)
	if err != nil {
		cmd.Fatal(err)
	}
	news := new(manga.NewsPost)
	if *newsU {
		resp, err := http.Get("http://" + core.Config.Remote + "/news")
		if err != nil {
			cmd.Fatalln("getting news:", err)
		}

		if resp.StatusCode >= 400 {
			e := new(Error)
			if err = json.NewDecoder(resp.Body).Decode(e); err != nil {
				cmd.Fatal(err)
			}
			cmd.Fatal(e)
		}
		if err = json.NewDecoder(resp.Body).Decode(news); err != nil {
			cmd.Fatal(err)
		}

		if _, err := file.Write([]byte(news.Body)); err != nil {
			cmd.Fatalln("writing post body:", err)
		}
		file.Seek(0, os.SEEK_SET)
	} else {
		if len(args) < 1 {
			cmd.Fatal("title required")
		}
		news.Title = strings.Join(args, " ")
	}

	// edit in $EDITOR
	util.Launch(util.GetEditor(), file.Name())

	// read and post it back
	buf := new(bytes.Buffer)
	if _, err = io.Copy(buf, file); err != nil {
		cmd.Fatalln("reading post body:", err)
	}

	news.Body = buf.String()

	buf.Reset()
	if err = json.NewEncoder(buf).Encode(news); err != nil {
		cmd.Fatal(err)
	}

	var endpoint string
	if *newsU {
		endpoint = "/news/update"
	} else {
		endpoint = "/news/create"
	}
	resp, err := http.Post("http://"+core.Config.Remote+endpoint, "application/json", buf)
	if err != nil {
		cmd.Fatalln("posting news:", err)
	}
	switch resp.StatusCode {
	case 201:
		json.NewDecoder(resp.Body).Decode(news)
		fmt.Printf("Created post #%d\n", news.Id)
		os.Remove(tmpPath)
	case 200:
		json.NewDecoder(resp.Body).Decode(news)
		fmt.Printf("Updated post #%d\n", news.Id)
		os.Remove(tmpPath)
	default:
		fmt.Println(resp.Status)
		e := new(Error)
		json.NewDecoder(resp.Body).Decode(e)
		cmd.Fatal(e)
	}
}
