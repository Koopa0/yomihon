GOLANGCI_LINT_VERSION := 2.13.2
GOSEC_VERSION := v2.28.0
STATICCHECK_VERSION := v0.8.1
ACTIONLINT_VERSION := v1.7.12
SHELLCHECK_VERSION := 0.11.0
GOVULNCHECK_VERSION := v1.5.0
BENCHSTAT_VERSION := v0.0.0-20260709024250-82a0b07e230d
DEADCODE_VERSION := v0.49.0
TAILWIND_VERSION := v4.1.17

BENCH_BASELINE ?= /tmp/yomihon-bench-baseline.txt
BENCH_CURRENT ?= /tmp/yomihon-bench-current.txt
COVER_PROFILE ?= /tmp/yomihon-coverage.out
COVER_SUMMARY ?= /tmp/yomihon-coverage.txt

# Root-module Go code lives in three owned trees. Listing those roots directly
# prevents an ignored frontend dependency with a stray Go package from entering
# the build graph before an exclusion can run.
#
# $(1) is any extra `go list` flags; the owned result lands in $$list.
define owned-go-list
set -eu; \
list=$$(go list $(1) ./assets ./cmd/... ./internal/...); \
[ -n "$$list" ] || { echo 'owned Go package list is empty' >&2; exit 1; }
endef

define require-go-tool
path=$$(command -v $(1)) || { echo '$(1) is required at $(3); run: make tools' >&2; exit 1; }; \
go version -m "$$path" | awk '$$1 == "mod" && $$2 == "$(2)" && $$3 == "$(3)" { found = 1 } END { exit !found }' || { \
	echo '$(1) must be built from $(2) $(3); run: make tools' >&2; \
	exit 1; \
}; \
built=$$(go version -m "$$path" | awk 'NR == 1 { sub(/^go/, "", $$2); print $$2 }'); \
needed=$$(awk '$$1 == "go" { print $$2; exit }' go.mod); \
[ "$$(printf '%s\n%s\n' "$$needed" "$$built" | sort -V | head -n 1)" = "$$needed" ] || { \
	echo "$(1) was built with go$$built and cannot read go$$needed source. The pinned version is right; the toolchain that built it is not, which is why the failure reads as a broken tool rather than a stale install. Run: make tools" >&2; \
	exit 1; \
}
endef

.PHONY: convention-check deadcode-check screenshots build build-check run test test-real-vault real-vault-build-check coverage-report bench-baseline bench-compare performance-smoke lint fmt fmt-check templ-fmt-check templ-gen-check vet staticcheck gosec vuln tools workflow-check tracked-paths-check mod-check frontend-check stylelint-check e2e-http-check fuzz-smoke browser-check mutation-check portable-build-check css css-check verify verify-spec clean

build: gen css
	go build -o bin/yomihon ./cmd/yomihon

build-check:
	go build ./assets ./cmd/... ./internal/...

run: gen css
	go run ./cmd/yomihon serve

test:
	@$(call owned-go-list); go test -race -count=1 -shuffle=on $$list

test-real-vault:
	@test -n "$${YOMIHON_ROOT:-}" || { echo 'YOMIHON_ROOT is required for test-real-vault' >&2; exit 2; }
	@$(call owned-go-list,-tags=realvault); go test -race -count=1 -tags=realvault $$list

# The ordinary gate compiles every opt-in real-vault test without selecting or
# reading an operator vault. Execution remains an explicit test-real-vault
# action, but build-tagged tests cannot silently rot between those runs.
real-vault-build-check:
	@$(call owned-go-list,-tags=realvault); YOMIHON_ROOT= go test -run='^$$' -tags=realvault $$list


coverage-report:
	@$(call owned-go-list); \
	coverpkg=$$(printf '%s\n' "$$list" | paste -sd, -); \
	testlog=$$(mktemp "$${TMPDIR:-/tmp}/yomihon-coverage-test.XXXXXX"); \
	trap 'rm -f "$$testlog"' EXIT HUP INT TERM; \
	if ! go test -count=1 -covermode=atomic -coverpkg="$$coverpkg" -coverprofile="$(COVER_PROFILE)" $$list >"$$testlog" 2>&1; then \
		cat "$$testlog"; \
		exit 1; \
	fi; \
	go tool cover -func="$(COVER_PROFILE)" >"$(COVER_SUMMARY)"; \
	tail -n 1 "$(COVER_SUMMARY)"

# Performance comparisons are deliberately local and opt-in. Absolute timings
# from shared CI runners are not a release gate; benchstat compares ten samples
# collected on the same machine, toolchain, and workload.
bench-baseline:
	@$(call owned-go-list); go test -run='^$$' -bench=. -benchmem -count=10 $$list > "$(BENCH_BASELINE)"
	@echo "benchmark baseline: $(BENCH_BASELINE)"

bench-compare:
	@$(call require-go-tool,benchstat,golang.org/x/perf,$(BENCHSTAT_VERSION))
	@test -f "$(BENCH_BASELINE)" || { echo 'run make bench-baseline before bench-compare' >&2; exit 2; }
	@$(call owned-go-list); go test -run='^$$' -bench=. -benchmem -count=10 $$list > "$(BENCH_CURRENT)"
	benchstat "$(BENCH_BASELINE)" "$(BENCH_CURRENT)"

lint:
	@version=$$(golangci-lint version); case "$$version" in *"version $(GOLANGCI_LINT_VERSION) "*) ;; *) echo 'golangci-lint $(GOLANGCI_LINT_VERSION) is required' >&2; exit 1;; esac
	golangci-lint config verify
	golangci-lint run ./assets ./cmd/... ./internal/...

fmt:
	@version=$$(golangci-lint version); case "$$version" in *"version $(GOLANGCI_LINT_VERSION) "*) ;; *) echo 'golangci-lint $(GOLANGCI_LINT_VERSION) is required' >&2; exit 1;; esac
	go tool templ fmt internal/ui
	go tool templ generate -path internal/ui
	golangci-lint fmt ./assets ./cmd ./internal

fmt-check: templ-fmt-check templ-gen-check
	@version=$$(golangci-lint version); case "$$version" in *"version $(GOLANGCI_LINT_VERSION) "*) ;; *) echo 'golangci-lint $(GOLANGCI_LINT_VERSION) is required' >&2; exit 1;; esac
	golangci-lint fmt --diff ./assets ./cmd ./internal

# templ's -fail mode formats in place before it returns 1, which makes it a
# poor verification primitive in a dirty checkout. Compare each formatter
# projection through a temporary file so this target is strictly read-only.
templ-fmt-check:
	@set -eu; \
	tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/yomihon-templ-fmt.XXXXXX"); \
	trap 'rm -rf "$$tmp"' 0 HUP INT TERM; \
	find internal/ui -type f -name '*.templ' -print | LC_ALL=C sort > "$$tmp/files"; \
	[ -s "$$tmp/files" ] || { echo 'templ source list is empty' >&2; exit 1; }; \
	while IFS= read -r file; do \
		go tool templ fmt -stdout "$$file" > "$$tmp/formatted"; \
		if ! cmp -s "$$file" "$$tmp/formatted"; then \
			diff -u "$$file" "$$tmp/formatted"; \
			exit 1; \
		fi; \
	done < "$$tmp/files"

# Generated templ Go is committed source for downstream Go tools. Keep this
# check read-only so verify cannot repair a stale projection before reviewing
# it, and bind generation to the only directory this repository owns.
templ-gen-check:
	go tool templ generate -check -path internal/ui

# The second pass compiles the yomihon_nodurable configuration, which also
# selects the platform-boundary test files hiding behind it on the supported
# platforms. Without it a break inside those files is invisible to every
# local gate: plain vet skips them and the cross-builds compile no tests.
vet:
	@$(call owned-go-list); go vet $$list
	@$(call owned-go-list); go vet -tags yomihon_nodurable $$list

staticcheck:
	@$(call require-go-tool,staticcheck,honnef.co/go/tools,$(STATICCHECK_VERSION))
	@$(call owned-go-list); staticcheck $$list

gosec:
	@$(call require-go-tool,gosec,github.com/securego/gosec/v2,$(GOSEC_VERSION))
	@$(call owned-go-list,-f '{{.Dir}}'); gosec -tests -nosec-require-rules -nosec-require-justification $$list

# Vulnerability data changes independently of the source tree, so this remains
# a separately readable CI result rather than making an advisory outage or a
# newly published report look like a deterministic source-test regression.
vuln:
	@$(call require-go-tool,govulncheck,golang.org/x/vuln,$(GOVULNCHECK_VERSION))
	@$(call owned-go-list); govulncheck $$list

tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLANGCI_LINT_VERSION)
	go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	go install github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)
	go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	go install golang.org/x/perf/cmd/benchstat@$(BENCHSTAT_VERSION)
	go install golang.org/x/tools/cmd/deadcode@$(DEADCODE_VERSION)

workflow-check:
	@$(call require-go-tool,actionlint,github.com/rhysd/actionlint,$(ACTIONLINT_VERSION))
	@sh tools/check-ci-tools.sh
	@shellcheck --version | awk '$$1 == "version:" && $$2 == "$(SHELLCHECK_VERSION)" { found = 1 } END { exit !found }' || { \
		echo 'ShellCheck $(SHELLCHECK_VERSION) is required' >&2; \
		exit 1; \
	}
	actionlint -pyflakes=
	shellcheck .github/e2e/*.sh tools/*.sh

# Repository hygiene, independent of any release process: a credential, private
# key, or generated database store must never be tracked.
tracked-paths-check:
	sh tools/check-tracked-paths.sh

gen:
	go tool templ generate -path internal/ui







css:
	tailwindcss -i assets/css/input.css -o assets/css/output.css --minify

css-check:
	@help=$$(NO_COLOR=1 tailwindcss --help 2>&1); case "$$help" in *"tailwindcss $(TAILWIND_VERSION)"*) ;; *) echo 'tailwindcss $(TAILWIND_VERSION) is required' >&2; exit 1;; esac
	@set -eu; \
	tmp=$$(mktemp "$${TMPDIR:-/tmp}/yomihon-css.XXXXXX"); \
	trap 'rm -f "$$tmp"' 0 HUP INT TERM; \
	tailwindcss -i assets/css/input.css -o "$$tmp" --minify >/dev/null; \
	if ! cmp -s assets/css/output.css "$$tmp"; then \
		diff -u assets/css/output.css "$$tmp"; \
		exit 1; \
	fi

mod-check:
	go mod tidy -diff
	go mod verify







frontend-check:
	npm ci --prefix .github --ignore-scripts --no-audit --fund=false
	npm exec --prefix .github -- biome lint --error-on-warnings assets/js/*.js .github/e2e/*.mjs
	@$(MAKE) --no-print-directory stylelint-check

# Regenerates the README's pictures from the example vault. The script sits
# beside .github/package.json rather than under e2e/, because everything in that
# directory is a probe the runner drives and a tool parked among them is a file
# nothing runs — which the probe runner refuses, correctly. A screenshot the
# repository can retake is a screenshot that can be kept current; one taken by
# hand goes stale the first time the interface moves and nobody can tell when.
# Each picture belongs to one README, so it opens a note written in that
# README's language: an English note under the English interface, a Traditional
# Chinese note under the Chinese one. The script refuses to write a picture
# where those two disagree.
screenshots:
	@set -eu; \
	tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/yomihon-shot.XXXXXX"); \
	trap 'rm -rf "$$tmp"' 0 HUP INT TERM; \
	go build -o "$$tmp/yomihon" ./cmd/yomihon; \
	YOMIHON_FIXTURE=examples/vault bash .github/e2e/serve.sh "$$tmp/yomihon" 19761 -- sh -c '\
	  LANG_CHOICE=en PAGE_PATH="/notes/Notes/What%20yomihon%20is.md" \
	    OUT=.github/media/reading-en.png node .github/screenshot.mjs && \
	  LANG_CHOICE=zh-Hant PAGE_PATH="/notes/Notes/中文/yomihon%20是什麼.md" \
	    OUT=.github/media/reading-zh-TW.png node .github/screenshot.mjs'

e2e-http-check:
	@set -eu; \
	tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/yomihon-e2e-http.XXXXXX"); \
	trap 'rm -rf "$$tmp"' 0 HUP INT TERM; \
	go build -o "$$tmp/yomihon" ./cmd/yomihon; \
	bash .github/e2e/serve.sh --self-test; \
	bash .github/e2e/smoke.sh --self-test; \
	bash .github/e2e/serve.sh "$$tmp/yomihon" 19733 -- bash .github/e2e/smoke.sh

# Each target gets a fixed work budget and one worker. A wall-clock fuzz budget
# is a poor merge gate: Go cancels the coordinator context at the deadline, so
# a saturated host can fail while an otherwise healthy worker is returning its
# final RPC. An execution budget is reproducible and still fails on any crash;
# race and deterministic-concurrency gates cover concurrency separately.
fuzz-smoke:
	@set -eu; \
	tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/yomihon-fuzz.XXXXXX"); \
	trap 'rm -rf "$$tmp"' 0 HUP INT TERM; \
	fuzz_cache="$$tmp/go-cache"; \
	mkdir -p "$$fuzz_cache"; \
	manifest="$$tmp/targets"; \
	$(call owned-go-list); \
	for pkg in $$list; do \
		go test "$$pkg" -run='^$$' -list='^Fuzz' | awk -v pkg="$$pkg" '/^Fuzz/ { print pkg "\t" $$1 }' >> "$$manifest"; \
	done; \
	[ -s "$$manifest" ] || { echo 'the owned module names no fuzz targets' >&2; exit 1; }; \
	cat "$$manifest"; \
	while IFS="$$(printf '\t')" read -r pkg target; do \
		out=$$(GOCACHE="$$fuzz_cache" go test "$$pkg" -run='^$$' -fuzz="^$${target}$$" -fuzztime=10000x -parallel=1 2>&1) || { printf '%s\n' "$$out"; exit 1; }; \
		printf '%s\n' "$$out"; \
		case "$$out" in *'no fuzz tests to fuzz'*) echo "$$pkg $$target explored no inputs" >&2; exit 1;; esac; \
	done < "$$manifest"

browser-check: frontend-check
	@set -eu; \
	tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/yomihon-browser.XXXXXX"); \
	trap 'rm -rf "$$tmp"' 0 HUP INT TERM; \
	go build -o "$$tmp/yomihon" ./cmd/yomihon; \
	bash .github/e2e/serve.sh "$$tmp/yomihon" 19734 -- bash .github/e2e/probes.sh

mutation-check: frontend-check
	@set -eu; \
	tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/yomihon-mutations.XXXXXX"); \
	trap 'rm -rf "$$tmp"' 0 HUP INT TERM; \
	go build -o "$$tmp/yomihon" ./cmd/yomihon; \
	bash .github/e2e/serve.sh "$$tmp/yomihon" 19735 -- bash .github/e2e/probes.sh --mutate

portable-build-check:
	@set -eu; \
	for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do \
		goos=$${target%/*}; \
		goarch=$${target#*/}; \
		echo "cross-build $$goos/$$goarch"; \
		CGO_ENABLED=0 GOOS="$$goos" GOARCH="$$goarch" go build ./assets ./cmd/... ./internal/...; \
	done

performance-smoke:
	@$(call owned-go-list); go test -run='^$$' -bench=. -benchtime=1x $$list

# Hand-written stylesheet sources live recursively under assets/css. output.css
# is the generated projection owned by the css/assets-drift gates, so lint the
# inputs that create it rather than the minified output. Build an ordered argv
# from a manifest so additions are discovered without relying on shell glob
# ordering or breaking paths that contain spaces.
stylelint-check:
	@set -eu; \
	tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/yomihon-stylelint.XXXXXX"); \
	trap 'rm -rf "$$tmp"' 0 HUP INT TERM; \
	find assets/css -type f -name '*.css' ! -path 'assets/css/output.css' -print > "$$tmp/unsorted"; \
	LC_ALL=C sort "$$tmp/unsorted" > "$$tmp/files"; \
	[ -s "$$tmp/files" ] || { echo 'stylesheet source list is empty' >&2; exit 1; }; \
	set --; \
	while IFS= read -r file; do set -- "$$@" "$$file"; done < "$$tmp/files"; \
	npm exec --prefix .github -- stylelint --config .stylelintrc.json --config-basedir .github "$$@"

# convention-check runs the checks that keep the module's shape from drifting
# while every individual change looks reasonable: the layer boundaries, the
# repeated message shapes, and the code nothing reaches any more. They are Go
# tests, so `make test` already runs them; naming them here gives the failure a
# name an operator can act on, and keeps the count of things `verify` asserts
# visible in one line.
convention-check: deadcode-check
	go test ./internal/archlock/ ./internal/sourcebytes/

# deadcode-check reports functions nothing reaches, counting every test as a
# caller. A function only its own tests reach is a real answer to a real
# question and stays; a function nothing reaches at all is weight a reader
# carries for nothing, and the report is empty today.
deadcode-check:
	@$(call require-go-tool,deadcode,golang.org/x/tools,$(DEADCODE_VERSION))
	@set -eu; \
	$(call owned-go-list); \
	found=$$(deadcode -test $$list); \
	[ -z "$$found" ] || { printf '%s\n' "$$found" >&2; echo 'nothing reaches the functions above; delete them or say in the code why they stay' >&2; exit 1; }

verify: tracked-paths-check mod-check fmt-check css-check vet lint staticcheck gosec vuln test convention-check real-vault-build-check workflow-check build-check frontend-check e2e-http-check fuzz-smoke browser-check mutation-check portable-build-check performance-smoke

verify-spec:
	@test -f tests/test-hooks.sh \
		-a -f tests/test-skill-format.sh \
		-a -f tests/test-consistency.sh || { \
		echo "ERROR: local go-spec harness is missing; install or refresh the bootstrap before verify-spec." >&2; \
		exit 1; \
	}
	@echo "=== Hook Tests ==="
	@bash tests/test-hooks.sh
	@echo ""
	@echo "=== Skill/Agent Format Tests ==="
	@bash tests/test-skill-format.sh
	@echo ""
	@echo "=== Consistency Tests ==="
	@bash tests/test-consistency.sh

clean:
	rm -rf bin tmp
