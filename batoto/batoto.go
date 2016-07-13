package batoto

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"ktkr.us/pkg/dn2/manga"
	"ktkr.us/pkg/manga/core"
	"ktkr.us/pkg/manga/util"

	"ktkr.us/pkg/stringdist"

	"github.com/PuerkitoBio/goquery"
)

var HTTPClient *http.Client

const (
	proxyURL              = "/proxy/request"
	batoto                = "https://www.bato.to"
	batotoSaveChapterPath = "/add_chapter?do=save"
	batotoUploadFilePath  = "/uploader/upload.php"
	batotoLoginPath       = "/forums/index.php?app=core&module=global&section=login&do=process"
	UA                    = "Mozilla/5.0 (Windows NT 6.3, WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/36.0.1985.125 Safari/537.36"
)

type ChapterUpload struct {
	Chap     *core.ChapSplit
	SeriesID string
	GroupID  string
	Archive  bool
	PostForm chan struct{}
}

// 1. upload to /uploader/upload.php
// 2. submit the form
// 3. ???
// 4. profit

/*
	Content-Type: multipart/form-data; boundary=...
	Origin: http://www.batoto.net
	Referer: http://www.batoto.net/add_chapter?comic=xxxx
	User-Agent: Mozilla/5.0 (Windows NT 6.3, WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/36.0.1985.125 Safari/537.36

	------
	Content-Disposition: form-data; name="Filename"

	whatever the file is.zip
	------
	Content-Disposition: form-data; name="instance"

	a15d728e6ee9b465b1b2a626e3adc9e0
	------
	Content-Disposition: form-data; name="SolmetraUploader"; filename="whatever the filename is.zip"
	Content-Type: application/octet-stream

	... (assuming binary data
	------
	Content-Disposition: form-data; name="Upload"

	Submit Query
	------
*/

func (b *ChapterUpload) Begin(p chan string) error {
	doArchive := "0"
	if b.Archive {
		doArchive = "1"
	}
	p <- "Getting uploader instance..."
	resp, err := HTTPClient.Get("http://bato.to/add_chapter")
	if err != nil {
		return err
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}

	//dumpDoc("add_chapter_"+b.Chap.Num, doc)

	instance, ok := doc.Find(`input[name="solmetraUploaderInstance"]`).First().Attr("value")
	if !ok {
		return errors.New("couldn't locate solmetra uploader instance ID")
	}

	p <- "Uploading file..."
	// first upload the file
	formReader, formWriter := io.Pipe()
	form := multipart.NewWriter(formWriter)

	var (
		zipName string
		zipPath string
		fi      os.FileInfo
	)

	if b.Chap.Id.Kind == manga.Volume {
		zipName = b.Chap.ZipName()
		zipPath = util.Rooted(zipName)
		fi, err = os.Stat(zipPath)
		if err != nil {
			return err
		}
	} else {
		fi, err = core.FirstArchive(b.Chap.Id)
		if err != nil {
			return err
		}
		zipName = fi.Name()
		zipPath = filepath.Join(core.TopLevel(), zipName)
	}
	totalSize := util.Bytes(fi.Size())

	/*
		instanceBytes := make([]byte, 16)
		rand.Read(instanceBytes)
		instance := hex.EncodeToString(instanceBytes)
	*/

	go func() {
		defer formWriter.CloseWithError(io.EOF)
		defer form.Close()

		form.WriteField("Filename", zipName)
		form.WriteField("instance", instance)
		form.WriteField("Upload", "Submit Query")

		f, err := os.Open(zipPath)
		if err != nil {
			log.Fatal(err)
		}
		w, err := form.CreateFormFile("SolmetraUploader", zipName)
		if err != nil {
			log.Fatal(err)
		}
		io.Copy(w, f)
	}()

	header := map[string]string{
		"Origin":     batoto,
		"Referer":    batoto + "/add_chapter?comic=" + b.SeriesID,
		"User-Agent": UA,
	}

	resp, err = util.UploadFileProgress("bato.to:80", batotoUploadFilePath, formReader, form, totalSize, header, p)
	if err != nil {
		return err
	}

	// check the response

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read file upload response: %v", err)
	}

	// returned status:
	//   "OK:<temporary generated filename for uploaded file>"
	// or
	//   "ERROR:<error code>"
	statusString := string(bytes.TrimSpace(buf))
	status := strings.Split(statusString, ":")
	if len(status) == 2 {
		switch status[0] {
		case "ERROR":
			return errors.New("solmetra: " + status[1])
		case "OK":
			// ok
		default:
			// do anything?
		}
	} else {
		return errors.New("solmetra: malformed return status: " + statusString)
	}

	p <- "Waiting..."

	<-b.PostForm
	defer func() {
		b.PostForm <- struct{}{}
	}()

	p <- "Posting form..."

	// now post the metadata

	formReader, formWriter = io.Pipe()
	form = multipart.NewWriter(formWriter)

	now := time.Now()
	rand.Seed(now.UnixNano())
	postKey := fmt.Sprintf("%x%x.%08d", now.Unix(), now.Nanosecond()/1000, rand.Intn(99999999))

	var vol string
	if b.Chap.Id.Kind == manga.Volume {
		vol = strconv.Itoa(b.Chap.Id.Ordinal)
	}

	dataId := fmt.Sprintf("zipfile|%s|%s", status[1], zipName)

	params := map[string]string{
		"edit_mode":   "0",          // ???
		"record":      "",           // ???
		"comic":       b.SeriesID,   // comic ID (select element is map of names to ID)
		"post_key":    postKey,      // seems to be unix || microtime in hex || '.' || random 8 digits
		"title":       b.Chap.Title, // chapter title
		"volume":      vol,          // volume #
		"chapter":     b.Chap.Num,   // chapter #
		"inc_chapter": "0",          // doesn't really matter
		"sort_order":  "",
		"p_group":     b.GroupID,
		"s_group":     "0",
		"t_group":     "0",
		"language":    "English",
		"remote":      "",
		"delay":       "0",
		"delay_mult":  "h",
		"archive":     doArchive,

		// How the server knows which file to associate with the form
		"solmetraUploaderInstance":               instance,
		"solmetraUploaderData[" + instance + "]": dataId,
		//"solmetraUploaderHijack_" + instance:     "y",
		//"solmetraUploaderRequired_" + instance:   "n",
	}

	go func() {
		defer formWriter.CloseWithError(io.EOF)
		defer form.Close()

		for name, field := range params {
			form.WriteField(name, field)
		}
	}()

	contentType := "multipart/form-data; boundary=" + form.Boundary()

	resp, err = HTTPClient.Post("http://bato.to/add_chapter?do=save", contentType, formReader)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		f, err := os.Create("response.html")
		resp.Write(f)
		f.Close()
		log.Print("server returned status ", resp.Status)
		if err != nil {
			log.Print("error writing response to disk: ", err)
		} else {
			log.Print("response written to 'response.html'")
		}
		return errors.New(resp.Status)
	}

	return nil
}

func Login() {
	var (
		resp    *http.Response
		scanner = bufio.NewScanner(os.Stdin)
		p       = make(url.Values)
	)

	batotoURL, err := url.Parse(batoto)
	if err != nil {
		log.Fatal(err)
	}

	p.Set("auth_key", "880ea6a14ea49e853634fbdc5015a024") // TODO, figure out what this is
	p.Set("referer", "http://bato.to/forums/")
	p.Set("rememberMe", "1")
	p.Set("anonymous", "0")

	for {
		fmt.Print("Batoto username: ")
		scanner.Scan()
		p.Set("ips_username", scanner.Text())

		fmt.Print("Batoto password: ")
		//p.Set("ips_password", getPassword(scanner))
		scanner.Scan()
		p.Set("ips_password", scanner.Text())

		resp, err = HTTPClient.PostForm(batoto+batotoLoginPath, p)
		if err != nil {
			log.Fatalln("Batoto login:", err)
		}

		if loginSuccessful(resp) {
			break
		}
	}

	cookies := resp.Cookies()
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatal(err)
	}
	jar.SetCookies(batotoURL, cookies)

	HTTPClient.Jar = jar
	fmt.Println("Logged in.")
}

// find the ids of the named series and group
func FindInfo(series, group string) (seriesID, groupID string, err error) {
	seriesID = core.Config.BatotoID
	groupID = core.Config.BatotoGroupID

	if seriesID != "" && groupID != "" {
		// we don't need to hit the page
		return core.Config.BatotoID, core.Config.BatotoGroupID, nil
	}

	// don't use goquery's http client because we need the login cookie for this page
	resp, err := HTTPClient.Get("http://bato.to/add_chapter")
	if err != nil {
		return "", "", err
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", err
	}

	//dumpDoc("upload.html", doc)

	name := ""

	if series != "" {
		currentDist := 0.0

		doc.Find("select#comic option").Each(func(i int, sel *goquery.Selection) {
			s := sel.Text()
			dist := match(s, series, false)
			if dist > currentDist {
				seriesID, _ = sel.Attr("value")
				name = s
				currentDist = dist
			}
		})

		if currentDist < 0.95 {
			return "", "", fmt.Errorf("Couldn't find series title '%s' (closest match was '%s', similarity %.3f)", series, name, currentDist)
		}
	}

	if group != "" {
		currentDist := 0.0

		doc.Find("select#p_group option").Each(func(i int, sel *goquery.Selection) {
			s := sel.Text()
			dist := match(s, group, false)
			if dist > currentDist {
				groupID, _ = sel.Attr("value")
				name = s
				currentDist = dist
			}
		})

		if currentDist < 0.9 {
			return "", "", fmt.Errorf("Couldn't find group name '%s' (closest match was '%s', similarity %.3f)", group, name, currentDist)
		}
	}

	core.Config.BatotoID = seriesID
	core.Config.BatotoGroupID = groupID
	core.SaveConfig()

	return
}

// match returns the Jaro-Winkler string distance between a and b. If strip is
// true, then punctuation and spaces will be stripped before the match is
// calculated.
func match(a, b string, strip bool) float64 {
	if strip {
		removePunct := func(ch rune) rune {
			if unicode.IsPunct(ch) || unicode.IsSpace(ch) {
				return '\x00'
			}
			return ch
		}
		a = strings.Replace(strings.Map(removePunct, a), "\x00", "", -1)
		b = strings.Replace(strings.Map(removePunct, b), "\x00", "", -1)
	}
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	return stringdist.JaroWinkler(a, b)
}

func loginSuccessful(resp *http.Response) bool {
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Fatalln("error reading login response body:", err)
	}

	msg := doc.Find("#content p.message.error").First()
	if len(msg.Nodes) == 0 {
		return true
	}

	ok, _ := regexp.MatchString("Username or password incorrect", msg.Text())
	return !ok
}

/*
func dumpDoc(path string, doc *goquery.Document) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	h, err := doc.Html()
	if err != nil {
		log.Fatal(err)
	}
	f.WriteString(h)
	f.Close()
}
*/
