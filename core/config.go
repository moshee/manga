package core

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

func init() {
	Config.Remote = os.Getenv("MANGA_REMOTE")
	Config.Group = os.Getenv("MANGA_GROUP")
	Config.DLServ = os.Getenv("MANGA_DLSERV")
}

var Config struct {
	Title         string `json:"-"`
	Shortname     string `json:",omitempty"`
	Id            int    `json:",omitempty"`
	Group         string `json:",omitempty"` // config will override env var
	Remote        string `json:"-"`
	DLServ        string `json:",omitempty"`
	BatotoID      string `json:",omitempty"`
	BatotoGroupID string `json:",omitempty"`
}

func LoadConfig() {
	wd := TopLevel()
	os.Chdir(wd)
	Config.Title = filepath.Base(wd)
	file, err := os.Open(".manga")
	if err != nil {
		log.Fatal(err)
	}
	if err = json.NewDecoder(file).Decode(&Config); err != nil {
		log.Fatal(".manga: ", err)
	}

	if Config.Group == "" {
		log.Fatal("env MANGA_GROUP or Group in .manga not set")
	}
}

func SaveConfig() {
	file, err := os.OpenFile(filepath.Join(TopLevel(), ".manga"), os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalln("Error saving .manga:", err)
	}

	if err = json.NewEncoder(file).Encode(&Config); err != nil {
		log.Fatalln("Error saving .manga:", err)
	}
}
