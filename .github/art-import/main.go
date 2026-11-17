//go:build ignore

// art-import turns the design team's delivered PNGs into the committed assets:
// one mask for each pictogram under assets/icons; the stills, the end ornament
// and the endpaper tiles under assets/art; and assets/css/art.css, which names
// each of them for a page to ask for.
//
// Run from the repository root:
//
//	go run .github/art-import/main.go [-accept] <art-in directory>
//
// The sources are docs/v2/art-in, which is not in version control, so nothing
// that gates a change reads them. inputs.sha256 beside this file names the
// bytes the committed outputs were made from; the program stops when an input
// differs, and -accept rewrites the list when the design team replaced a file
// on purpose.
//
// It lives under .github with the other development tools, and only the
// standard library is used, so no module is added. The write-path test of
// internal/archlock walks every .go file outside hidden directories, ignoring
// build tags, and permits writes only in internal/mark.
//
// Every step is integer arithmetic over bits and the PNG encoder is the
// standard library's, so the same inputs give the same bytes on every machine
// that builds with the toolchain go.mod names.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The grid of pictograms.md §4: each sheet cell is 1024 px, the shape lives in
// the live square, and the design leaves a 96 px band around that square, which
// is the 24-unit grid the sizes 16, 20 and 24 px are named for. Every cell is
// cut to the same window, so a stop square stays smaller than a play triangle.
const (
	cell       = 1024
	liveMin    = 224
	liveMax    = 800
	band       = 96
	windowMin  = liveMin - band // 128
	windowMax  = liveMax + band // 896
	iconPixels = 96             // the 768 px window divided by 8: a unit is 4 px, a band 12
)

// The still is shown 240 px wide on a desktop and 160 px on a phone, so 480 px
// serves a 2× screen and a 3× phone. The ornament is shown 64 px wide.
const (
	stillW, stillH = 480, 300
	ornamentW      = 512
	ornamentH      = 128
	// A tile is shown with its short side at 32 px (system.md §11); a short
	// side of 128 keeps 4× for sharper screens.
	tileShort = 128
	tileShown = 32
)

type icon struct{ file, design string }

type sheet struct {
	file       string
	cols, rows int
	icons      []icon
}

// The file name is the designers' name in lowercase with hyphens, except where
// the vocabulary in AGENTS.md has another word: 讀完 is finished. The design's
// own name is kept beside it so a file can be traced back to its sheet.
var sheets = []sheet{
	{"sheet-marks.png", 4, 2, []icon{
		{"mark", "My marks"}, {"highlight", "Highlight"}, {"question", "Question"}, {"thought", "Thought"},
		{"bookmark", "Bookmark"}, {"bookmark-off", "Bookmark, not set"}, {"finished", "Done"}, {"finished-off", "Done, not set"},
	}},
	{"sheet-kinds.png", 4, 3, []icon{
		{"note", "Note"}, {"draft", "Draft"}, {"folder", "Folder"}, {"course", "Course"},
		{"unit", "Unit"}, {"index", "Index"}, {"drill", "Drill"}, {"journal", "Journal"},
		{"report", "Report"}, {"template", "Template"}, {"picture", "Picture"}, {"unreadable", "Unreadable"},
	}},
	{"sheet-controls.png", 4, 5, []icon{
		{"play", "Play"}, {"pause", "Pause"}, {"stop", "Stop"}, {"previous", "Previous"},
		{"next", "Next"}, {"speaker", "Speaker"}, {"search", "Search"}, {"settings", "Settings"},
		{"copy", "Copy"}, {"close", "Close"}, {"sidebar", "Sidebar"}, {"external-link", "External link"},
		{"back", "Back"}, {"forward", "Forward"}, {"disclosure", "Disclosure"}, {"ruby", "Reading aid (ruby)"},
		{"filter", "Filter"}, {"more", "More"}, {"undo", "Undo"}, {"shuffle", "Shuffle"},
	}},
	{"sheet-findings.png", 3, 1, []icon{
		{"error", "Error"}, {"warning", "Warning"}, {"broken-link", "Broken link"},
	}},
}

var stills = []string{"desk-first", "stand-empty", "catalogue-empty", "not-found", "unreadable", "reading"}

const endpapers = 12

// bitmap is a picture of exactly two colours: ink is 1 where the source is black.
type bitmap struct {
	w, h int
	ink  []uint8
}

func (b *bitmap) at(x, y int) uint8 { return b.ink[y*b.w+x] }

// made is one output file.
type made struct {
	dir, name string
	data      []byte
}

func main() {
	accept := flag.Bool("accept", false, "record the inputs' checksums instead of checking them")
	flag.Parse()
	if flag.NArg() != 1 {
		fail(errors.New("usage: go run .github/art-import/main.go [-accept] <art-in directory>"))
	}
	if _, err := os.Stat("go.mod"); err != nil {
		fail(errors.New("run this from the repository root"))
	}
	if err := run(flag.Arg(0), *accept); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "art-import:", err)
	os.Exit(1)
}

func run(src string, accept bool) error {
	for _, s := range sheets {
		if s.cols*s.rows != len(s.icons) {
			return fmt.Errorf("%s: %d cells and %d names", s.file, s.cols*s.rows, len(s.icons))
		}
	}
	inputs := inputNames()
	raw := make(map[string][]byte, len(inputs))
	for _, name := range inputs {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			return err
		}
		raw[name] = data
	}
	if err := checkInputs(raw, inputs, accept); err != nil {
		return err
	}

	var outputs []made
	var rules []string
	for _, s := range sheets {
		b, err := decode(s.file, raw[s.file])
		if err != nil {
			return err
		}
		if b.w != s.cols*cell || b.h != s.rows*cell {
			return fmt.Errorf("%s is %d×%d, want %d×%d", s.file, b.w, b.h, s.cols*cell, s.rows*cell)
		}
		for i, ic := range s.icons {
			cx, cy := (i%s.cols)*cell, (i/s.cols)*cell
			if err := insideTheLiveSquare(s.file, ic.design, b, cx, cy); err != nil {
				return err
			}
			data, err := encode(reduce(b, cx+windowMin, cy+windowMin, cx+windowMax, cy+windowMax, iconPixels, iconPixels))
			if err != nil {
				return err
			}
			outputs = append(outputs, made{"icons", ic.file + ".png", data})
			rules = append(rules, rule(fmt.Sprintf(".y-icon[data-icon='%s']", ic.file), "icons/"+ic.file+".png", ""))
		}
	}
	for _, n := range stills {
		file := "still-" + n + ".png"
		b, err := decode(file, raw[file])
		if err != nil {
			return err
		}
		if b.w != 2400 || b.h != 1500 {
			return fmt.Errorf("%s is %d×%d, want 2400×1500", file, b.w, b.h)
		}
		data, err := encode(reduce(b, 0, 0, b.w, b.h, stillW, stillH))
		if err != nil {
			return err
		}
		outputs = append(outputs, made{"art", file, data})
		rules = append(rules, rule(fmt.Sprintf(".y-still[data-still='%s']", n), "art/"+file, ""))
	}
	{
		const file = "ornament-end.png"
		b, err := decode(file, raw[file])
		if err != nil {
			return err
		}
		if b.w != 4096 || b.h != 1024 {
			return fmt.Errorf("%s is %d×%d, want 4096×1024", file, b.w, b.h)
		}
		data, err := encode(reduce(b, 0, 0, b.w, b.h, ornamentW, ornamentH))
		if err != nil {
			return err
		}
		outputs = append(outputs, made{"art", file, data})
		rules = append(rules, rule(".y-ornament", "art/"+file, ""))
	}
	for n := 1; n <= endpapers; n++ {
		name := fmt.Sprintf("endpaper-%02d", n)
		b, err := decode(name+".png", raw[name+".png"])
		if err != nil {
			return err
		}
		ow, oh := tileSize(b.w, b.h)
		data, err := encode(reduce(b, 0, 0, b.w, b.h, ow, oh))
		if err != nil {
			return err
		}
		outputs = append(outputs, made{"art", name + ".png", data})
		// The tile keeps its own proportions with its short side at tileShown,
		// so nothing is stretched whatever the tile's shape.
		size := fmt.Sprintf("auto %dpx", tileShown)
		if b.w < b.h {
			size = fmt.Sprintf("%dpx auto", tileShown)
		}
		rules = append(rules, rule(fmt.Sprintf(".y-endpaper[data-endpaper='%s']", name), "art/"+name+".png", size))
	}

	return write(outputs, rules)
}

// inputNames is every file the program reads, in the order it reads them.
func inputNames() []string {
	var names []string
	for _, s := range sheets {
		names = append(names, s.file)
	}
	for _, n := range stills {
		names = append(names, "still-"+n+".png")
	}
	names = append(names, "ornament-end.png")
	for n := 1; n <= endpapers; n++ {
		names = append(names, fmt.Sprintf("endpaper-%02d.png", n))
	}
	return names
}

const inputList = ".github/art-import/inputs.sha256"

// checkInputs compares each input with the list committed beside the program,
// or writes the list when the inputs are being accepted.
func checkInputs(raw map[string][]byte, names []string, accept bool) error {
	sorted := slices.Sorted(slices.Values(names))
	var list strings.Builder
	sums := make(map[string]string, len(sorted))
	for _, n := range sorted {
		sum := sha256.Sum256(raw[n])
		sums[n] = hex.EncodeToString(sum[:])
		fmt.Fprintf(&list, "%s  %s\n", sums[n], n)
	}
	if accept {
		return os.WriteFile(inputList, []byte(list.String()), 0o644)
	}
	recorded, err := os.ReadFile(inputList)
	if err != nil {
		return fmt.Errorf("%w; run with -accept to record these inputs", err)
	}
	if string(recorded) == list.String() {
		return nil
	}
	want := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSuffix(string(recorded), "\n"), "\n") {
		if hash, name, ok := strings.Cut(line, "  "); ok {
			want[name] = hash
		}
	}
	var differ []string
	for _, n := range sorted {
		if want[n] != sums[n] {
			differ = append(differ, n)
		}
	}
	return fmt.Errorf("%s differs from %s; run with -accept if the design team replaced it on purpose", strings.Join(differ, ", "), inputList)
}

// decode reads a PNG of exactly two colours. A third colour, or a grey, means
// anti-aliasing got into a source the design promised without any.
func decode(name string, data []byte) (*bitmap, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	r := img.Bounds()
	b := &bitmap{w: r.Dx(), h: r.Dy(), ink: make([]uint8, r.Dx()*r.Dy())}
	for y := range b.h {
		for x := range b.w {
			c := color.NRGBA64Model.Convert(img.At(r.Min.X+x, r.Min.Y+y)).(color.NRGBA64)
			switch {
			case c.A != 0xffff:
				return nil, fmt.Errorf("%s: pixel (%d,%d) is not opaque", name, x, y)
			case c.R == 0 && c.G == 0 && c.B == 0:
				b.ink[y*b.w+x] = 1
			case c.R == 0xffff && c.G == 0xffff && c.B == 0xffff:
			default:
				return nil, fmt.Errorf("%s: pixel (%d,%d) is neither black nor white", name, x, y)
			}
		}
	}
	return b, nil
}

// insideTheLiveSquare holds a cell to what pictograms.md promises: some ink,
// and none outside the live square. A shape that crossed it would be cut by
// the window, or by the next cell.
func insideTheLiveSquare(file, design string, b *bitmap, cx, cy int) error {
	minX, minY, maxX, maxY := cell, cell, -1, -1
	for y := range cell {
		for x := range cell {
			if b.at(cx+x, cy+y) == 1 {
				minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
			}
		}
	}
	switch {
	case maxX < 0:
		return fmt.Errorf("%s: the cell for %q is empty", file, design)
	case minX < liveMin || minY < liveMin || maxX >= liveMax || maxY >= liveMax:
		return fmt.Errorf("%s: %q has ink at x %d–%d, y %d–%d, outside the live square %d–%d", file, design, minX, maxX, minY, maxY, liveMin, liveMax-1)
	}
	return nil
}

// tileSize scales a tile so its short side is tileShort and the other side is
// the nearest whole pixel.
func tileSize(w, h int) (int, int) {
	short := min(w, h)
	scale := func(n int) int { return (n*tileShort + short/2) / short }
	return scale(w), scale(h)
}

// reduce averages the window [x0,x1)×[y0,y1) of b down to ow×oh, as ink
// coverage in the alpha channel of a black picture. Each output pixel covers
// sw/ow source columns and sh/oh rows, in whole and fractional pixels, and the
// weights are exact integers: an output pixel is covered by sw·sh units in all.
func reduce(b *bitmap, x0, y0, x1, y1, ow, oh int) *image.NRGBA {
	sw, sh := x1-x0, y1-y0
	total := uint64(sw) * uint64(sh)
	out := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	for oy := range oh {
		fy, wy := weights(oy, sh, oh)
		for ox := range ow {
			fx, wx := weights(ox, sw, ow)
			var area uint64
			for j, vy := range wy {
				for i, vx := range wx {
					if b.at(x0+fx+i, y0+fy+j) == 1 {
						area += vx * vy
					}
				}
			}
			out.SetNRGBA(ox, oy, color.NRGBA{A: uint8((255*area + total/2) / total)})
		}
	}
	return out
}

// weights lists, for output cell o of n over a source of s pixels, the first
// source pixel it touches and how much of each one it covers, in units of 1/n
// of a source pixel; they add up to s.
func weights(o, s, n int) (first int, w []uint64) {
	lo, hi := o*s, (o+1)*s
	first = lo / n
	for i := first; i*n < hi; i++ {
		w = append(w, uint64(min((i+1)*n, hi)-max(i*n, lo)))
	}
	return first, w
}

func encode(img *image.NRGBA) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func rule(selector, url, size string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s {\n  mask-image: url('/static/%s');\n", selector, url)
	if size != "" {
		fmt.Fprintf(&b, "  mask-size: %s;\n", size)
	}
	b.WriteString("}\n")
	return b.String()
}

const cssHeader = `/* Generated by .github/art-import/main.go from the design team's delivered art;
   do not edit. One rule for each name a page may ask for; the rules every one
   of them shares are in components.css. */
`

// write replaces the two asset directories' pictures, their checksum lists and
// the stylesheet. A picture the program does not make is refused rather than
// deleted: a file that is there was put there by someone.
func write(outputs []made, rules []string) error {
	byDir := map[string][]made{}
	for _, o := range outputs {
		byDir[o.dir] = append(byDir[o.dir], o)
	}
	for _, dir := range []string{"icons", "art"} {
		path := filepath.Join("assets", dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
		have, err := filepath.Glob(filepath.Join(path, "*.png"))
		if err != nil {
			return err
		}
		known := map[string]bool{}
		for _, o := range byDir[dir] {
			known[o.name] = true
		}
		for _, h := range have {
			if !known[filepath.Base(h)] {
				return fmt.Errorf("%s is not a picture this program makes; remove it first", h)
			}
		}
		var sums strings.Builder
		for _, o := range slices.SortedFunc(slices.Values(byDir[dir]), func(a, b made) int { return strings.Compare(a.name, b.name) }) {
			if err := os.WriteFile(filepath.Join(path, o.name), o.data, 0o644); err != nil {
				return err
			}
			sum := sha256.Sum256(o.data)
			fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(sum[:]), o.name)
		}
		if err := os.WriteFile(filepath.Join(path, "SHA256SUMS"), []byte(sums.String()), 0o644); err != nil {
			return err
		}
	}
	css := cssHeader + "\n" + strings.Join(rules, "\n")
	if err := os.WriteFile(filepath.Join("assets", "css", "art.css"), []byte(css), 0o644); err != nil {
		return err
	}
	fmt.Printf("art-import: %d pictures, %d rules\n", len(outputs), len(rules))
	return nil
}
