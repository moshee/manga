package dn

import (
	"encoding/ascii85"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"ktkr.us/pkg/manga/core"
	"ktkr.us/pkg/manga/util"

	"ktkr.us/pkg/dn2/manga"

	"golang.org/x/crypto/sha3"
)

const (
	uploadPath = "/upload"
	hashSize   = 64
)

var hashEncSize = ascii85.MaxEncodedLen(hashSize)

func UploadFile(filePath string) (*http.Response, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	shake := sha3.NewShake256()
	_, err = io.Copy(shake, f)
	if err != nil {
		return nil, err
	}
	f.Seek(0, os.SEEK_SET)

	h := make([]byte, hashSize)
	shake.Read(h)
	buf := base64.URLEncoding.EncodeToString(h)

	q := url.Values{}
	q.Add("Name", fi.Name())
	q.Add("Shake256", url.QueryEscape(string(buf)))
	path := uploadPath + "?" + q.Encode()

	return util.UploadFileProgress(core.Config.DLServ, path, f, nil, util.Bytes(fi.Size()), nil, nil)
}

func PostForm(hostPort, reqPath string, files map[string]string, r *manga.Release) (*http.Response, error) {
	// add :http to the host string if it isn't there
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		host = net.JoinHostPort(hostPort, "http")
	} else {
		host = hostPort
	}
	log.Print("remote host: ", host)

	formReader, formWriter := io.Pipe()
	form := multipart.NewWriter(formWriter)

	var totalSize util.Bytes

	if files != nil {
		for _, filename := range files {
			fi, err := os.Stat(filename)
			if err != nil {
				log.Fatalf("postForm: %v", err)
			}
			totalSize += util.Bytes(fi.Size())
		}
	}

	go func() {
		defer formWriter.CloseWithError(io.EOF)
		defer form.Close()
		w, err := form.CreateFormField("data")
		if err != nil {
			log.Fatal(err)
		}

		if err = json.NewEncoder(w).Encode(r); err != nil {
			log.Fatal(err)
		}

		if files != nil {
			for field, filename := range files {
				file, err := os.Open(filename)
				if err != nil {
					log.Fatalf("postForm: %v", err)
				}
				defer file.Close()

				w, err := form.CreateFormFile(field, filepath.Base(filename))
				if err != nil {
					log.Fatalf("postForm: %v", err)
				}
				io.Copy(w, file)
			}
		}
	}()

	return util.UploadFileProgress(host, reqPath, formReader, form, totalSize, nil, nil)
}
