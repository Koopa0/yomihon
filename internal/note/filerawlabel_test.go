package note_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestFileInfoRawLinkMatchesDelivery(t *testing.T) {
	t.Parallel()
	files := []struct {
		name       string
		data       []byte
		attachment bool
	}{
		{"large.html", []byte("<!doctype html>" + strings.Repeat(" ", render.MaxSourceBytes)), true},
		{"large.xml", []byte("<?xml version=\"1.0\"?>" + strings.Repeat(" ", render.MaxSourceBytes)), true},
		{"large.bin", make([]byte, render.MaxSourceBytes+1), false},
		{"small.bin", []byte{0, 1, 2}, false},
	}
	for _, file := range files {
		for _, lang := range []string{"zh-Hant", "en"} {
			t.Run(file.name+"/"+lang, func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				if err := os.WriteFile(filepath.Join(root, file.name), file.data, 0o600); err != nil {
					t.Fatalf("WriteFile(%q): %v", file.name, err)
				}
				server := newServer(t, root)
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/notes/"+file.name, http.NoBody)
				if err != nil {
					t.Fatalf("NewRequestWithContext(%q): %v", file.name, err)
				}
				req.Header.Set("Cookie", wording.CookieName+"="+lang)
				resp, err := server.Client().Do(req)
				if err != nil {
					t.Fatalf("GET /notes/%s: %v", file.name, err)
				}
				page, err := io.ReadAll(resp.Body)
				closeErr := resp.Body.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("read/close page: %v / %v", err, closeErr)
				}
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("GET /notes/%s status = %d", file.name, resp.StatusCode)
				}
				label := "開啟原始位元組"
				if lang == "en" {
					label = "Open the raw bytes"
				}
				if file.attachment {
					label = "下載檔案"
					if lang == "en" {
						label = "Download the file"
					}
				}
				want := []string{"/raw/" + file.name, label}
				if diff := cmp.Diff(want, fileInfoRawLink(t, page)); diff != "" {
					t.Errorf("caught: file-info raw link disagrees with delivery (-want +got):\n%s", diff)
				}
				status, header, data := fetch(t, server.Client(), server.URL+"/raw/"+file.name)
				if status != http.StatusOK {
					t.Fatalf("GET /raw/%s status = %d", file.name, status)
				}
				t.Log("invoked: file-info raw link and raw delivery")
				if got := strings.HasPrefix(header.Get("Content-Disposition"), "attachment;"); got != file.attachment {
					t.Errorf("raw attachment = %v, want %v", got, file.attachment)
				}
				if !bytes.Equal([]byte(data), file.data) {
					t.Error("raw file bytes changed")
				}
			})
		}
	}
}

func fileInfoRawLink(t *testing.T, page []byte) []string {
	t.Helper()
	z := html.NewTokenizer(bytes.NewReader(page))
	var links [][]string
	var link []string
	for {
		switch z.Next() {
		case html.ErrorToken:
			if err := z.Err(); !errors.Is(err, io.EOF) {
				t.Fatalf("parse file page: %v", err)
			}
			if len(links) != 1 {
				t.Fatalf("file-info raw link count = %d, want 1", len(links))
			}
			return links[0]
		case html.StartTagToken:
			token := z.Token()
			if token.Data != "a" {
				continue
			}
			var class, href string
			for _, attr := range token.Attr {
				switch attr.Key {
				case "class":
					class = attr.Val
				case "href":
					href = attr.Val
				}
			}
			if strings.Contains(" "+class+" ", " y-fileinfo__raw ") {
				link = []string{href, ""}
			}
		case html.TextToken:
			if link != nil {
				link[1] += z.Token().Data
			}
		case html.EndTagToken:
			if link != nil && z.Token().Data == "a" {
				link[1] = strings.TrimSpace(link[1])
				links = append(links, link)
				link = nil
			}
		case html.SelfClosingTagToken, html.CommentToken, html.DoctypeToken:
		}
	}
}
