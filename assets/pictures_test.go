package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image/png"
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// The committed pictures, the stylesheet that names them and the checksum lists
// are made together by .github/art-import/main.go from sources that are not in
// version control, so nothing here can run it. What can be held is that the
// three agree with one another and with what the program promises.

// artRule is one rule of the generated stylesheet: the kind of picture, the
// name its selector carries (none for the ornament), and the file it draws.
var artRule = regexp.MustCompile(`(?m)^\.y-(icon|still|endpaper|ornament)(?:\[data-[a-z]+='([a-z0-9-]+)'\])? \{\n  mask-image: url\('/static/((?:icons|art)/[a-z0-9-]+\.png)'\);`)

func TestEveryPictureHasARuleAndEveryRuleAPicture(t *testing.T) {
	t.Parallel()

	css, err := Files.ReadFile("css/art.css")
	if err != nil {
		t.Fatalf("read css/art.css: %v", err)
	}
	drawn := map[string]bool{}
	for _, m := range artRule.FindAllSubmatch(css, -1) {
		kind, name, file := string(m[1]), string(m[2]), string(m[3])
		// What the selector names must be what the rule draws, or a page asking
		// for one picture would get another.
		want := map[string]string{
			"icon":     "icons/" + name + ".png",
			"still":    "art/still-" + name + ".png",
			"endpaper": "art/" + name + ".png",
			"ornament": "art/ornament-end.png",
		}[kind]
		if file != want {
			t.Errorf("a %s rule naming %q draws %s, want %s", kind, name, file, want)
		}
		if kind == "endpaper" && !strings.HasPrefix(name, "endpaper-") {
			t.Errorf("an endpaper rule names %q, which is not an endpaper", name)
		}
		if drawn[file] {
			t.Errorf("%s is drawn by two rules", file)
		}
		drawn[file] = true
	}
	if n := strings.Count(string(css), "mask-image:"); n != len(drawn) {
		t.Errorf("art.css has %d mask-image declarations and %d rules in the shape a page selects by", n, len(drawn))
	}

	for _, dir := range []string{"icons", "art"} {
		entries, err := fs.ReadDir(Files, dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".png") {
				continue
			}
			file := dir + "/" + entry.Name()
			if !drawn[file] {
				t.Errorf("%s has no rule in art.css, so no page can ask for it", file)
			}
			delete(drawn, file)
		}
	}
	for file := range drawn {
		t.Errorf("art.css draws %s, which is not embedded", file)
	}
}

func TestCommittedPicturesMatchTheirChecksumLists(t *testing.T) {
	t.Parallel()

	for _, dir := range []string{"icons", "art"} {
		list, err := Files.ReadFile(dir + "/SHA256SUMS")
		if err != nil {
			t.Fatalf("read %s/SHA256SUMS: %v", dir, err)
		}
		listed := map[string]string{}
		for line := range strings.SplitSeq(strings.TrimSuffix(string(list), "\n"), "\n") {
			hash, name, ok := strings.Cut(line, "  ")
			if !ok || len(hash) != 64 {
				t.Fatalf("%s/SHA256SUMS line %q is not \"<sha256>  <file>\"", dir, line)
			}
			listed[name] = hash
		}
		entries, err := fs.ReadDir(Files, dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		pictures := 0
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".png") {
				continue
			}
			pictures++
			data, err := Files.ReadFile(dir + "/" + entry.Name())
			if err != nil {
				t.Fatalf("read %s/%s: %v", dir, entry.Name(), err)
			}
			sum := sha256.Sum256(data)
			if got, want := hex.EncodeToString(sum[:]), listed[entry.Name()]; got != want {
				t.Errorf("%s/%s SHA-256 = %s, listed %q; the pictures are made by .github/art-import/main.go and not edited by hand", dir, entry.Name(), got, want)
			}
		}
		if pictures != len(listed) {
			t.Errorf("%s/SHA256SUMS lists %d files and the directory holds %d pictures", dir, len(listed), pictures)
		}
	}
}

// An icon is the 768 px window of its cell reduced to 96 px, so the design's
// live square, 3 to 21 units, is 12 to 84 px, and its ink never leaves it. A cut
// taken from the wrong place would break this for most of them.
func TestEveryIconIsOneMaskWithItsInkInsideTheLiveSquare(t *testing.T) {
	t.Parallel()

	entries, err := fs.ReadDir(Files, "icons")
	if err != nil {
		t.Fatalf("read icons: %v", err)
	}
	icons := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".png") {
			continue
		}
		icons++
		data, err := Files.ReadFile("icons/" + entry.Name())
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Errorf("%s: %v", entry.Name(), err)
			continue
		}
		if b := img.Bounds(); b.Dx() != 96 || b.Dy() != 96 {
			t.Errorf("%s is %d×%d, want 96×96", entry.Name(), b.Dx(), b.Dy())
			continue
		}
		minX, minY, maxX, maxY := 96, 96, -1, -1
		for y := range 96 {
			for x := range 96 {
				r, g, b, a := img.At(x, y).RGBA()
				if a == 0 {
					continue
				}
				if r|g|b != 0 {
					t.Fatalf("%s: pixel (%d,%d) is not black, so the colour a page gives it would be mixed with another", entry.Name(), x, y)
				}
				minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
			}
		}
		if maxX < 0 {
			t.Errorf("%s has no ink", entry.Name())
		} else if minX < 12 || minY < 12 || maxX > 83 || maxY > 83 {
			t.Errorf("%s has ink at x %d–%d, y %d–%d, outside the live square 12–83", entry.Name(), minX, maxX, minY, maxY)
		}
	}
	if icons != 43 {
		t.Errorf("assets/icons holds %d pictograms, want the design's 43", icons)
	}
}
