module github.com/koopa0/yomihon

go 1.27.0

toolchain go1.27.4

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/a-h/templ v0.3.1100
	// chroma has no stable v3 tag yet; this is deliberately the newest v3
	// prerelease, and the pin moves to v3.0.0 when it exists. Dropping to the
	// stable v2 line would be an API rework without cause; the render goldens
	// gate any future bump.
	github.com/alecthomas/chroma/v3 v3.0.0-alpha.5
	github.com/google/go-cmp v0.7.0
	github.com/tats-u/goldmark-cjk-friendly/v2 v2.0.3
	github.com/yuin/goldmark v1.8.7
	go.yaml.in/yaml/v3 v3.0.6
	golang.org/x/mod v0.43.0
	golang.org/x/net v0.62.0
	golang.org/x/sys v0.50.0
	golang.org/x/text v0.44.0
)

require (
	// Held at the version templ's own go.mod requires: the newer a-h/parse
	// changes its Parser interface and the templ tool stops compiling, so
	// `go get -u` must not move it until templ does.
	github.com/a-h/parse v0.0.0-20250122154542-74294addb73e // indirect
	github.com/andybalholm/brotli v1.3.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cli/browser v1.3.0 // indirect
	github.com/dlclark/regexp2/v2 v2.13.0 // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/natefinch/atomic v1.0.1 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
)

tool github.com/a-h/templ/cmd/templ
