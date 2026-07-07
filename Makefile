MODULE := github.com/koopa0/kurodo

# The whole-tree Go targets run over a filtered list that drops the ignored
# node_modules tree the frontend linters leave behind: it can carry a stray Go
# package, and an unfiltered ./... would build, vet, and format it as if it
# were ours. GO_PKGS feeds the package-based targets; GO_DIRS feeds goimports,
# which takes paths. The guards turn an empty list — a broken toolchain or a
# filter that matched everything — into a hard failure instead of a silent
# pass over nothing.
GO_PKGS := $(shell go list ./... | grep -v /node_modules/)
GO_DIRS := $(shell go list -f '{{.Dir}}' ./... | grep -v /node_modules/)

require-pkgs = test -n "$(strip $(GO_PKGS))" || { echo 'go package list is empty after filtering node_modules' >&2; exit 1; }
require-dirs = test -n "$(strip $(GO_DIRS))" || { echo 'go directory list is empty after filtering node_modules' >&2; exit 1; }

.PHONY: build run test lint fmt vet gen css verify clean

build: gen css
	go build -o bin/kurodo ./cmd/kurodo

run: gen css
	go run ./cmd/kurodo serve

test:
	@$(require-pkgs)
	go test -race -count=1 -shuffle=on $(GO_PKGS)

lint:
	golangci-lint config verify
	golangci-lint run

fmt:
	@$(require-dirs)
	goimports -w -local $(MODULE) $(GO_DIRS)

vet:
	@$(require-pkgs)
	go vet $(GO_PKGS)

gen:
	go tool templ generate

css:
	tailwindcss -i assets/css/input.css -o assets/css/output.css --minify

verify: fmt vet lint test build

clean:
	rm -rf bin tmp
