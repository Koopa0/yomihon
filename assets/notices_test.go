package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The release ships THIRD_PARTY_NOTICES.md beside every binary, because the
// licences of what is compiled into the binary ask for their notice to travel
// with it. These checks hold that file to the binary: what the toolchain links
// and what the vendored bundle carries are read from their own sources, so a
// dependency or a Mermaid bump that the notices do not follow fails here.

// goStdlibHeading is the notices section for the Go standard library and
// runtime, which no module listing reports.
const goStdlibHeading = "Go standard library and runtime"

// releaseTargets is the release surface: the two 64-bit architectures on each
// supported reader OS. It is spelled out, not read from `go tool dist list`,
// for the reason cmd/yomihon spells out the same matrix for its environment
// wall: a newly supported platform has to be an intentional review. The module
// set can differ between them (golang.org/x/sys is not linked on Windows), and
// the notices cover the union.
var releaseTargets = []struct{ goos, goarch string }{
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"windows", "amd64"},
	{"windows", "arm64"},
}

// licenceFileName matches the files a project publishes its licence terms in.
// A module's root is read for every file this matches, so a NOTICE or a PATENTS
// grant that travels with a licence is held to the same rule as the licence.
var licenceFileName = regexp.MustCompile(`(?i)^(licen[cs]e|copying|notice|patents|copyright|unlicense)([._-].*)?$`)

// linkedModule is one third-party module the toolchain links into yomihon.
type linkedModule struct {
	version string
	dir     string
}

func readThirdPartyNotices(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile("../THIRD_PARTY_NOTICES.md")
	if err != nil {
		t.Fatalf("read third-party notices: %v", err)
	}
	return string(data)
}

// squashSpace reduces a text to its words, so that a licence reproduced in a
// Markdown fence still matches the file it was taken from when the two differ
// in line endings, trailing spaces or wrapping.
func squashSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// fenceOpening returns the backtick run that opens a fenced code block on line,
// or "" when the line opens none.
func fenceOpening(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	run := trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, "`"))]
	if len(run) < 3 {
		return ""
	}
	return run
}

var noticeHeading = regexp.MustCompile(`^#{1,6}[ \t]+(.+?)[ \t]*$`)

// noticeSections splits the notices at their headings, keyed by heading text.
// A heading-looking line inside a fenced block is licence text, not a heading:
// the yaml.v3 licence has several.
func noticeSections(notices string) map[string]string {
	sections := make(map[string]string)
	var heading string
	var body strings.Builder
	flush := func() {
		if heading != "" {
			sections[heading] += body.String()
		}
		body.Reset()
	}
	var open string
	for line := range strings.Lines(notices) {
		text := strings.TrimRight(line, "\r\n")
		switch {
		case open != "":
			if strings.Trim(strings.TrimSpace(text), "`") == "" && len(strings.TrimSpace(text)) >= len(open) {
				open = ""
			}
		case fenceOpening(text) != "":
			open = fenceOpening(text)
		default:
			if m := noticeHeading.FindStringSubmatch(text); m != nil {
				flush()
				heading = m[1]
				continue
			}
		}
		body.WriteString(line)
	}
	flush()
	return sections
}

// licenceFilesIn reads every licence file at the root of dir.
func licenceFilesIn(t *testing.T, dir string) map[string]string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	files := make(map[string]string)
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !licenceFileName.MatchString(entry.Name()) {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name())) // #nosec G304 -- a file listed from a module's own directory
		if readErr != nil {
			t.Fatalf("read %s: %v", filepath.Join(dir, entry.Name()), readErr)
		}
		files[entry.Name()] = string(data)
	}
	return files
}

// linkedGoModules asks the toolchain which third-party modules each release
// target links. The set comes from the linker's own listing rather than from
// go.mod, which also names modules that only tests or build tools use: those
// are not in the binary and owe it nothing.
func linkedGoModules(t *testing.T) map[string]linkedModule {
	t.Helper()

	const format = `{{with .Module}}{{if not .Main}}{{.Path}}{{"\t"}}{{.Version}}{{"\t"}}{{.Dir}}{{"\n"}}{{end}}{{end}}`
	modules := make(map[string]linkedModule)
	for _, target := range releaseTargets {
		cmd := exec.CommandContext(t.Context(), "go", "list", "-deps", "-f", format, "./cmd/yomihon") // #nosec G204 -- fixed Go invocation; the format and target are this file's own constants
		cmd.Dir = ".."
		cmd.Env = append(os.Environ(), "GOOS="+target.goos, "GOARCH="+target.goarch, "CGO_ENABLED=0")
		out, err := cmd.Output()
		if err != nil {
			if exit, ok := errors.AsType[*exec.ExitError](err); ok {
				t.Fatalf("go list -deps for %s/%s: %v\n%s", target.goos, target.goarch, err, exit.Stderr)
			}
			t.Fatalf("go list -deps for %s/%s: %v", target.goos, target.goarch, err)
		}
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			fields := strings.Split(line, "\t")
			if len(fields) != 3 || fields[2] == "" {
				t.Fatalf("go list -deps for %s/%s printed %q, want <path>, <version>, <directory>; is the module cache populated?", target.goos, target.goarch, line)
			}
			modules[fields[0]] = linkedModule{version: fields[1], dir: fields[2]}
		}
	}
	return modules
}

func goRoot(t *testing.T) string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "go", "env", "GOROOT") // #nosec G204 -- fixed Go invocation
	cmd.Dir = ".."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go env GOROOT: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// requireReproduced fails for each licence file whose text is not in body.
func requireReproduced(t *testing.T, where, body string, files map[string]string) {
	t.Helper()

	if len(files) == 0 {
		t.Errorf("%s: no licence file was found to hold the notice against; read its licence by hand and decide how this check should treat it", where)
		return
	}
	squashed := squashSpace(body)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		if !strings.Contains(squashed, squashSpace(files[name])) {
			t.Errorf("%s does not reproduce its %s", where, name)
		}
	}
}

// Every module the binary links has a section in the notices, and the section
// reproduces the licence files that module publishes at the version in go.mod.
// The first half is the issue's rule; the second is what stops a section that
// exists from being a heading over the wrong text. Both are read from the
// module cache, so a bump that changes a licence turns this red until the
// notices follow it, while a bump that does not change one costs nothing.
func TestThirdPartyNoticesReproduceEveryLinkedGoModule(t *testing.T) {
	t.Parallel()

	sections := noticeSections(readThirdPartyNotices(t))
	modules := linkedGoModules(t)
	if len(modules) == 0 {
		t.Fatal("the toolchain reported no third-party module linked into cmd/yomihon, so every check below would pass for the wrong reason")
	}
	for _, path := range slices.Sorted(maps.Keys(modules)) {
		module := modules[path]
		body, ok := sections[path]
		if !ok {
			t.Errorf("%s %s is linked into the binary, but THIRD_PARTY_NOTICES.md has no section headed %q", path, module.version, path)
			continue
		}
		requireReproduced(t, "the section for "+path, body, licenceFilesIn(t, module.dir))
	}

	body, ok := sections[goStdlibHeading]
	if !ok {
		t.Fatalf("THIRD_PARTY_NOTICES.md has no section headed %q for the standard library and runtime every binary is built from", goStdlibHeading)
	}
	requireReproduced(t, "the section for the Go standard library and runtime", body, licenceFilesIn(t, goRoot(t)))
}

// A licence banner the vendored bundle carries, as esbuild kept it.
type mermaidBanner struct {
	file string // the chunk that carries it
	pkg  string // the package its entry names
	text string // the comment as written
}

var (
	// esbuild gathers the comments it was told to keep into one block per file,
	// each entry headed by the path of the file it came from.
	bannerBlock  = regexp.MustCompile(`(?s)/\*! Bundled license information:\n(.*?)\n\*/`)
	bannerHeader = regexp.MustCompile(`^(\S+):$`)
)

// bannerPackage is the npm package a banner entry's path names.
func bannerPackage(path string) string {
	parts := strings.Split(path, "/")
	if strings.HasPrefix(parts[0], "@") && len(parts) > 1 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// vendoredMermaidBanners reads every licence banner out of the vendored bytes,
// and reports each file in which an @license tag stands outside a recognised
// banner block, because a tag the parse does not reach is one nobody checked.
func vendoredMermaidBanners(t *testing.T) (banners []mermaidBanner, stray []string) {
	t.Helper()

	err := fs.WalkDir(Files, "js/mermaid", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(name, ".mjs") {
			return nil
		}
		data, readErr := Files.ReadFile(name)
		if readErr != nil {
			return readErr
		}
		src := string(data)
		tagsInBlocks := 0
		for _, block := range bannerBlock.FindAllStringSubmatch(src, -1) {
			tagsInBlocks += strings.Count(block[0], "@license")
			start := len(banners)
			for line := range strings.Lines(block[1]) {
				line = strings.TrimRight(line, "\r\n")
				if m := bannerHeader.FindStringSubmatch(line); m != nil {
					banners = append(banners, mermaidBanner{file: name, pkg: bannerPackage(m[1])})
				} else if len(banners) > start {
					banners[len(banners)-1].text += line + "\n"
				}
			}
		}
		if strings.Count(src, "@license") != tagsInBlocks {
			stray = append(stray, name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read the vendored Mermaid bundle: %v", err)
	}
	return banners, stray
}

// bannerWords reduces a banner, or the notices, to the words of its text: the
// comment delimiters esbuild wraps a banner in and the stars a block comment
// indents its lines with are not part of what the banner says.
func bannerWords(s string) string {
	var words []string
	for word := range strings.FieldsSeq(s) {
		switch word {
		case "(*!", "(**", "*)", "*":
			continue
		}
		words = append(words, word)
	}
	return strings.Join(words, " ")
}

// mermaidBundledPackages is every package the vendored Mermaid 11.15.0 bundle
// compiles in, as name|version|licence, with the version and licence column the
// notices table gives it. The bundle is minified, so no listing of this set
// exists in the vendored bytes: it was read from the source maps that the npm
// packages mermaid@11.15.0 and @mermaid-js/parser@1.1.1 publish, as the notices
// say. Holding the table to it is what keeps a package, or the licence beside
// it, from disappearing from the notices unseen; the manifest check below is
// what makes a new Mermaid release read it again.
const mermaidBundledPackages = `
@braintree/sanitize-url|7.1.1|MIT
@chevrotain/gast|11.1.1|Apache-2.0
@chevrotain/regexp-to-ast|11.1.1|Apache-2.0
@chevrotain/utils|11.1.1|Apache-2.0
@iconify/utils|3.0.2|MIT
@mermaid-js/parser|1.1.1|MIT
@upsetjs/venn.js|2.0.0|MIT
chevrotain|11.1.1|Apache-2.0
chevrotain-allstar|0.3.1|MIT
cose-base|1.0.3|MIT
cose-base|2.2.0|MIT
cytoscape|3.33.1|MIT
cytoscape-cose-bilkent|4.1.0|MIT
cytoscape-fcose|2.2.0|MIT
d3-array|2.12.1|BSD-3-Clause
d3-array|3.2.4|ISC
d3-axis|3.0.0|ISC
d3-brush|3.0.0|ISC
d3-color|3.1.0|ISC
d3-dispatch|3.0.1|ISC
d3-ease|3.0.1|BSD-3-Clause
d3-format|3.1.0|ISC
d3-hierarchy|3.1.2|ISC
d3-interpolate|3.0.1|ISC
d3-path|1.0.9|BSD-3-Clause
d3-path|3.1.0|ISC
d3-sankey|0.12.3|BSD-3-Clause
d3-scale|4.0.2|ISC
d3-scale-chromatic|3.1.0|ISC; Apache-2.0 (ColorBrewer)
d3-selection|3.0.0|ISC
d3-shape|1.3.7|BSD-3-Clause
d3-shape|3.2.0|ISC
d3-time|3.1.0|ISC
d3-time-format|4.1.0|ISC
d3-timer|3.0.1|ISC
d3-transition|3.0.1|ISC
d3-zoom|3.0.0|ISC
dagre-d3-es|7.0.14|MIT
dayjs|1.11.19|MIT
dompurify|3.4.0|Apache-2.0 OR MPL-2.0
es-toolkit|1.45.1|MIT
internmap|2.0.3|ISC
js-yaml|4.1.1|MIT
katex|0.16.25|MIT
khroma|2.1.0|MIT
langium|4.2.0|MIT
layout-base|1.0.2|MIT
layout-base|2.0.1|MIT
lodash-es|4.17.23|MIT
lodash-es|4.18.1|MIT
marked|16.3.0|MIT; BSD-3-Clause (Markdown)
roughjs|4.6.6|MIT
stylis|4.3.6|MIT
ts-dedent|2.2.0|MIT
uuid|14.0.0|MIT
vscode-jsonrpc|8.2.0|MIT
vscode-languageserver-protocol|3.17.5|MIT
vscode-languageserver-textdocument|1.0.12|MIT
vscode-languageserver-types|3.17.5|MIT
vscode-uri|3.1.0|MIT
`

// mermaidEmbeddedComponents is the code that those packages' own prebuilt files
// carry from other projects, as component|held in. Each is named in the notices
// by the table that follows the packages.
const mermaidEmbeddedComponents = `
heap|cytoscape 3.33.1
lodash|cytoscape 3.33.1
Embeddable Minimum Strictly-Compliant Promises/A+ Thenable 1.1.1|cytoscape 3.33.1
Event object based on jQuery events|cytoscape 3.33.1
Bezier curve function generator|cytoscape 3.33.1
Runge-Kutta spring physics function generator|cytoscape 3.33.1
hachure-fill|roughjs 4.6.6
path-data-parser|roughjs 4.6.6
points-on-curve|roughjs 4.6.6
points-on-path|roughjs 4.6.6
fmin|@upsetjs/venn.js 2.0.0
path-browserify|vscode-uri 3.1.0
`

// The notices name every component the vendored Mermaid bundle carries. Two
// things are read from the bundle itself: each licence banner it keeps, whose
// package must be listed and whose words must be reproduced, and the manifest
// that locks the whole bundle. The rest of the inventory has no listing in the
// minified bytes, so it is pinned above and tied to this bundle by the
// manifest's hash.
func TestThirdPartyNoticesNameTheMermaidBundle(t *testing.T) {
	t.Parallel()

	notices := readThirdPartyNotices(t)
	words := bannerWords(notices)

	banners, stray := vendoredMermaidBanners(t)
	if len(banners) == 0 {
		t.Fatal("no licence banner was found in the vendored Mermaid bundle, so the checks over its banners would pass for the wrong reason")
	}
	for _, name := range stray {
		t.Errorf("%s has an @license tag outside any \"Bundled license information\" block, so this check did not read it; extend the parse", name)
	}
	for _, banner := range banners {
		if !strings.Contains(notices, "| "+banner.pkg+" | ") {
			t.Errorf("%s carries a licence banner for %s, which the notices do not list among the packages bundled inside Mermaid", banner.file, banner.pkg)
		}
		if !strings.Contains(words, bannerWords(banner.text)) {
			t.Errorf("the licence banner of %s in %s is not reproduced in the notices:\n%s", banner.pkg, banner.file, banner.text)
		}
	}

	for line := range strings.SplitSeq(strings.TrimSpace(mermaidBundledPackages), "\n") {
		if row := "| " + strings.ReplaceAll(line, "|", " | ") + " |"; !strings.Contains(notices, row) {
			t.Errorf("the notices' table of packages bundled inside Mermaid has no row %q", row)
		}
	}
	for line := range strings.SplitSeq(strings.TrimSpace(mermaidEmbeddedComponents), "\n") {
		if row := "| " + strings.ReplaceAll(line, "|", " | ") + " | "; !strings.Contains(notices, row) {
			t.Errorf("the notices' table of code embedded in those packages has no row starting %q", row)
		}
	}

	manifest, err := Files.ReadFile("js/mermaid/SHA256SUMS")
	if err != nil {
		t.Fatalf("read Mermaid manifest: %v", err)
	}
	sum := sha256.Sum256(manifest)
	if hash := hex.EncodeToString(sum[:]); !strings.Contains(notices, hash) {
		t.Errorf("the Mermaid bundle has changed: assets/js/mermaid/SHA256SUMS now hashes to %s, which the notices do not record. "+
			"Read the package inventory again from the new release's source maps, update the notices and the two lists in this file, then record the new hash", hash)
	}
}
