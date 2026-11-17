package asset

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The pictures are served as the fonts are: every .png under assets/icons and
// assets/art is in the closed set with an image type, and what sits beside them
// (the checksum list) is not. The counts are the design's full list
// (docs/v2/design/images.md): 43 pictograms, and 6 stills, the ornament and 12
// endpapers.
func TestEveryPictureIsServedAndNothingBesideThemIs(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	Register(mux)
	for dir, want := range map[string]int{"icons": 43, "art": 19} {
		entries, err := os.ReadDir(filepath.Join("..", "..", "assets", dir))
		if err != nil {
			t.Fatalf("ReadDir(%s) error = %v", dir, err)
		}
		pictures, beside := 0, 0
		for _, entry := range entries {
			name := dir + "/" + entry.Name()
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/"+name, http.NoBody))
			if !strings.HasSuffix(entry.Name(), ".png") {
				beside++
				if response.Code != http.StatusNotFound {
					t.Errorf("GET /static/%s status = %d, want %d: only the pictures are meant to be fetchable", name, response.Code, http.StatusNotFound)
				}
				continue
			}
			pictures++
			if response.Code != http.StatusOK {
				t.Errorf("GET /static/%s status = %d, want %d", name, response.Code, http.StatusOK)
			}
			if got := response.Header().Get("Content-Type"); got != "image/png" {
				t.Errorf("GET /static/%s Content-Type = %q, want image/png", name, got)
			}
			if !HasPicture(dir, strings.TrimSuffix(entry.Name(), ".png")) {
				t.Errorf("HasPicture(%q, %q) = false for a picture that is served", dir, entry.Name())
			}
		}
		if pictures != want {
			t.Errorf("assets/%s holds %d pictures, want %d", dir, pictures, want)
		}
		if beside == 0 {
			t.Errorf("nothing sits beside the pictures in assets/%s, so the half about not serving it asserts nothing", dir)
		}
	}
}

// A name a page could write with a slash, a dot or another kind of file in it
// is not a picture, whatever the closed set holds under that spelling.
func TestHasPictureAnswersForTheNameAlone(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		dir, name string
		want      bool
	}{
		{"icons", "bookmark", true},
		{"icons", "finished", true},
		{"art", "still-desk-first", true},
		{"art", "endpaper-12", true},
		{"art", "ornament-end", true},
		{"icons", "done", false},
		{"icons", "bookmark.png", false},
		{"icons", "../art/still-desk-first", false},
		{"icons", "", false},
		{"icons", "SHA256SUMS", false},
		{"fonts", "Geist-Variable", false},
	} {
		if got := HasPicture(tc.dir, tc.name); got != tc.want {
			t.Errorf("HasPicture(%q, %q) = %v, want %v", tc.dir, tc.name, got, tc.want)
		}
	}
}
