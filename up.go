package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"ktkr.us/pkg/dn2/manga"

	"ktkr.us/pkg/manga/batoto"
	"ktkr.us/pkg/manga/core"
	"ktkr.us/pkg/manga/dn"
	"ktkr.us/pkg/manga/job"
	"ktkr.us/pkg/manga/util"
)

var cmdUp = &Command{
	Name:    "up",
	Summary: "[-m <message>] [-isbn <ISBN>] [-nsfw] [-b] <identifier>",
	Help: `
Upload archives and update databases.`,
	Flags: flag.NewFlagSet("up", flag.ExitOnError),
}

func init() {
	batoto.HTTPClient = httpClient
	cmdUp.Run = doUp
}

var (
	upM    = cmdUp.Flags.String("m", "", "Release message")
	upISBN = cmdUp.Flags.String("isbn", "", "ISBN")
	upNSFW = cmdUp.Flags.Bool("nsfw", false, "Mark release as NSFW")
	upMeta = cmdUp.Flags.Bool("meta", false, "Only post metadata")

	upB        = cmdUp.Flags.Bool("b", false, "Upload to Batoto")
	upBArchive = cmdUp.Flags.Bool("archive", false, "Flag Batoto chapter as archived")
	upBTitle   = cmdUp.Flags.String("t", "", "Chapter title (for single chapters)")

	httpClient = new(http.Client)
)

func doUp(cmd *Command, args []string) {
	core.LoadConfig()
	id := cmd.identifier(args[0])
	if core.Config.Id == 0 {
		cmdUp.Fatal("manga up: no series id set in .manga")
	}

	if *upB {
		batotoUpload(id)
	} else {
		displaynoneUpload(id)
	}
}

func displaynoneUpload(id core.Identifier) {
	if core.Config.Remote == "" {
		cmdUp.Fatal("displaynone remote url not set")
	}

	archiveFi, err := core.FirstArchive(id)
	if err != nil {
		cmdUp.Fatal("displaynone: ", err)
	}

	r := &manga.Release{
		SeriesId: core.Config.Id,
		Kind:     id.Kind,
		Ordinal:  id.Ordinal,
		Filename: archiveFi.Name(),
		Filesize: manga.Filesize(archiveFi.Size()),
		NSFW:     *upNSFW,
		ISBN:     *upISBN,
	}

	const releasemsgName = "MANGA-RELEASEMSG"

	if *upM == "" {
		util.Launch(util.GetEditor(), releasemsgName)
		tmpdata, err := ioutil.ReadFile(releasemsgName)
		if err != nil {
			if !os.IsNotExist(err) {
				cmdUp.Fatalln("displaynone upload: error reading release notes:", err)
			}
		} else {
			*upM = string(tmpdata)
		}
	}

	defer os.Remove(releasemsgName)

	r.Notes = *upM

	/*
		files := map[string]string{
			"archive": rooted(r.Filename),
		}
	*/
	files := make(map[string]string)

	if id.Kind != manga.Chapter {
		files["cover"] = util.Rooted(fmt.Sprintf("%s-%s.jpg", core.Config.Shortname, id))
		files["thumb"] = util.Rooted(fmt.Sprintf("%s-%s-thumb.jpg", core.Config.Shortname, id))
	}

	if !(*upMeta) {
		cmdUp.Println("uploading archive to displaynone...")
		resp, err := dn.UploadFile(util.Rooted(r.Filename))
		if err != nil {
			cmdUp.Fatalln("displaynone upload:", err)
		}
		if resp.StatusCode != http.StatusCreated {
			cmdUp.Println("displaynone upload: expected status 201, got", resp.Status)
			var e struct{ Error string }
			if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
				cmdUp.Fatalln("failed decoding response:", err)
			} else {
				cmdUp.Fatalln("server responded:", e)
			}
		}
	}
	cmdUp.Println("posting metadata...")
	resp, err := dn.PostForm(core.Config.Remote, "/release/create", files, r)
	if err != nil {
		cmdUp.Fatalln("displaynone upload:", err)
	}

	defer resp.Body.Close()

	fmt.Println(resp.Status)

	if resp.StatusCode != http.StatusCreated {
		var e Error
		json.NewDecoder(resp.Body).Decode(&e)
		cmdUp.Fatalln("displaynone upload:", e)
	}

	json.NewDecoder(resp.Body).Decode(r)
	cmdUp.Printf("created release #\033[1m%d\033[0m (%s %v).", r.Id, core.Config.Title, id)
}

func batotoUpload(id core.Identifier) {
	var chaps []*core.ChapSplit
	if id.Kind == manga.Volume {
		chaps = core.ParseSplits(id)
	} else {
		chaps = []*core.ChapSplit{
			&core.ChapSplit{
				Id:    id,
				Num:   strconv.Itoa(id.Ordinal),
				Title: *upBTitle,
			},
		}
	}

	batoto.Login()
	seriesID, groupID, err := batoto.FindInfo(core.Config.Title, core.Config.Group)
	if err != nil {
		cmdUp.Fatalln("findInfo:", err)
	}

	t := make(job.Group, len(chaps))
	chans := make([]chan struct{}, len(t))
	for i, chap := range chaps {
		chans[i] = make(chan struct{})
		j := &batoto.ChapterUpload{chap, seriesID, groupID, *upBArchive, chans[i]}
		t[i] = job.New(j, chap.String())
	}

	// upload files concurrently
	go func() {
		if err := t.Begin(); err != nil {
			cmdUp.Fatal(err)
		}
	}()

	// post forms serially
	for _, ch := range chans {
		ch <- struct{}{}
		<-ch
	}
}
