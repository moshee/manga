package util

import (
	"bufio"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"time"
)

func UploadFileProgress(host, reqPath string, formReader io.Reader, form *multipart.Writer,
	totalSize Bytes, header map[string]string, p chan string) (*http.Response, error) {

	haveChan := true
	if p == nil {
		p = make(chan string)
		haveChan = false
	}

	req, err := http.NewRequest("POST", reqPath, formReader)
	if err != nil {
		return nil, err
	}
	req.Host = host
	if _, ok := formReader.(*os.File); ok {
		req.ContentLength = int64(totalSize)
	}
	if form != nil {
		req.Header.Set("Content-Type", "multipart/form-data; boundary="+form.Boundary())
	}
	if header != nil {
		for k, v := range header {
			req.Header.Set(k, v)
		}
	}

	conn, err := net.Dial("tcp", host)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	sw := NewStatWriter(conn, 16, 250*time.Millisecond, totalSize, p)
	done := make(chan error)

	go func() {
		done <- req.Write(sw)
	}()

	if !haveChan {
		go func() {
			for s := range p {
				fmt.Fprintf(os.Stderr, "\033[J%s\033[0G", s)
			}
			fmt.Fprintln(os.Stderr)
		}()
	}

	err = sw.Report(done)
	if err != nil {
		return nil, err
	}

	r := bufio.NewReader(conn)
	return http.ReadResponse(r, req)
}
