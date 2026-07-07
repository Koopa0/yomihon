MODULE := github.com/koopa0/kurodo

# `go list ./...` over the whole tree also descends into node_modules — the
# ignored frontend build tree, which can carry a stray Go package — and an
# unfiltered list would build, vet, and format it as if it were ours. Each Go
# target filters it out inside its own recipe (via filtered-go-list) so the
# listing runs only for the target that needs it, not on every `make`. The
# listing is captured in two steps: the `go list` first, so a broken toolchain
# aborts the recipe with go's own error, then the filter, so an empty result
# after filtering fails loudly instead of passing silently over nothing.
#
# $(1) is any extra `go list` flags; the filtered result lands in $$list.
define filtered-go-list
set -eu; \
list=$$(go list $(1) ./...); \
list=$$(printf '%s\n' "$$list" | grep -vE '/node_modules(/|$$)' || true); \
[ -n "$$list" ] || { echo 'go list is empty after filtering node_modules' >&2; exit 1; }
endef

.PHONY: build run test lint fmt vet gen css verify clean

build: gen css
	go build -o bin/kurodo ./cmd/kurodo

run: gen css
	go run ./cmd/kurodo serve

test:
	@$(call filtered-go-list); go test -race -count=1 -shuffle=on $$list

lint:
	golangci-lint config verify
	golangci-lint run

fmt:
	@$(call filtered-go-list,-f '{{.Dir}}'); goimports -w -local $(MODULE) $$list

vet:
	@$(call filtered-go-list); go vet $$list

gen:
	go tool templ generate

css:
	tailwindcss -i assets/css/input.css -o assets/css/output.css --minify

verify: fmt vet lint test build

clean:
	rm -rf bin tmp
