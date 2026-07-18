GOLANGCI_LINT_VERSION := 2.12.2
GOSEC_VERSION := v2.28.0
SQLC_VERSION := v1.31.1
STATICCHECK_VERSION := v0.7.0
ACTIONLINT_VERSION := v1.7.12
SHELLCHECK_VERSION := 0.11.0
GOVULNCHECK_VERSION := v1.5.0
BENCHSTAT_VERSION := v0.0.0-20260709024250-82a0b07e230d
TAILWIND_VERSION := v4.1.17

BENCH_BASELINE ?= /tmp/yomihon-bench-baseline.txt
BENCH_CURRENT ?= /tmp/yomihon-bench-current.txt
COVER_PROFILE ?= /tmp/yomihon-coverage.out
COVER_SUMMARY ?= /tmp/yomihon-coverage.txt
SOURCE_ARCHIVE ?= dist/candidates/yomihon-$(RELEASE_VERSION).tar.gz

# Root-module Go code lives in three owned trees. Listing those roots directly
# prevents an ignored frontend dependency with a stray Go package from entering
# the build graph before an exclusion can run. The SQLite bake-off is a nested
# module: tools-check gates the selected driver, while CI additionally runs the
# retained comparison through tools-check-mattn.
#
# $(1) is any extra `go list` flags; the owned result lands in $$list.
define owned-go-list
set -eu; \
list=$$(go list $(1) ./assets ./cmd/... ./internal/...); \
[ -n "$$list" ] || { echo 'owned Go package list is empty' >&2; exit 1; }
endef

define require-go-tool
path=$$(command -v $(1)) || { echo '$(1) is required at $(3)' >&2; exit 1; }; \
go version -m "$$path" | awk '$$1 == "mod" && $$2 == "$(2)" && $$3 == "$(3)" { found = 1 } END { exit !found }' || { \
	echo '$(1) must be built from $(2) $(3)' >&2; \
	exit 1; \
}
endef

.PHONY: build build-check run test test-real-vault real-vault-build-check provider-live coverage-report bench-baseline bench-compare performance-smoke lint fmt fmt-check templ-fmt-check templ-gen-check vet staticcheck gosec vuln tools workflow-check policy-check license-check source-archive-candidate source-artifact source-artifact-check gen sqlc sqlc-check sqlc-version mod-check tools-check-prepare tools-check tools-check-mattn frontend-check stylelint-check e2e-http-check fuzz-smoke browser-check mutation-check portable-build-check status-activation-mutations css css-check verify verify-spec clean

build: gen css
	go build -o bin/yomihon ./cmd/yomihon

build-check: sqlc-check
	go build ./assets ./cmd/... ./internal/...

run: gen css
	go run ./cmd/yomihon serve

test: sqlc-check
	@$(call owned-go-list); go test -race -count=1 -shuffle=on $$list

test-real-vault:
	@test -n "$${YOMIHON_ROOT:-}" || { echo 'YOMIHON_ROOT is required for test-real-vault' >&2; exit 2; }
	@$(call owned-go-list,-tags=realvault); go test -race -count=1 -tags=realvault $$list

# The ordinary gate compiles every opt-in real-vault test without selecting or
# reading an operator vault. Execution remains an explicit test-real-vault
# action, but build-tagged tests cannot silently rot between those runs.
real-vault-build-check:
	@$(call owned-go-list,-tags=realvault); YOMIHON_ROOT= go test -run='^$$' -tags=realvault $$list

# Deliberately absent from ordinary PR verification: this sends only D57's
# fixed synthetic protocol probes, but it spends provider quota. Unlike a
# skipped test, the named certification gate refuses to pass without the
# explicit opt-in and the operator's own credential.
provider-live:
	@test "$${YOMIHON_EMBED_LIVE:-}" = 1 || { echo 'YOMIHON_EMBED_LIVE=1 is required for provider-live' >&2; exit 2; }
	@test -n "$${YOMIHON_EMBED_KEY:-}" || { echo 'YOMIHON_EMBED_KEY is required for provider-live' >&2; exit 2; }
	go test -count=1 -run='^TestGeminiEmbedding2LiveProtocol$$' ./internal/search/semantic

# Coverage is an observable report, not a percentage gate. A repository-wide
# floor would reward shallow tests and punish generated or platform-specific
# code without proving any contract; watched-red and mutation evidence remain
# the acceptance standard. CI retains both the machine profile and this exact
# function summary so missing areas can still be reviewed over time.
coverage-report: sqlc-check
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

lint: sqlc-check
	@version=$$(golangci-lint version); case "$$version" in *"version $(GOLANGCI_LINT_VERSION) "*) ;; *) echo 'golangci-lint $(GOLANGCI_LINT_VERSION) is required' >&2; exit 1;; esac
	golangci-lint config verify
	golangci-lint run ./assets ./cmd/... ./internal/...

fmt:
	@version=$$(golangci-lint version); case "$$version" in *"version $(GOLANGCI_LINT_VERSION) "*) ;; *) echo 'golangci-lint $(GOLANGCI_LINT_VERSION) is required' >&2; exit 1;; esac
	go tool templ fmt internal/ui
	go tool templ generate -path internal/ui
	golangci-lint fmt ./assets ./cmd ./internal
	cd tools/sqlite-driver-bakeoff && golangci-lint fmt --config ../../.golangci.yml

fmt-check: templ-fmt-check templ-gen-check
	@version=$$(golangci-lint version); case "$$version" in *"version $(GOLANGCI_LINT_VERSION) "*) ;; *) echo 'golangci-lint $(GOLANGCI_LINT_VERSION) is required' >&2; exit 1;; esac
	golangci-lint fmt --diff ./assets ./cmd ./internal
	cd tools/sqlite-driver-bakeoff && golangci-lint fmt --config ../../.golangci.yml --diff

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

vet: sqlc-check
	@$(call owned-go-list); go vet $$list

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
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	go install github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)
	go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	go install golang.org/x/perf/cmd/benchstat@$(BENCHSTAT_VERSION)

workflow-check:
	@$(call require-go-tool,actionlint,github.com/rhysd/actionlint,$(ACTIONLINT_VERSION))
	@shellcheck --version | awk '$$1 == "version:" && $$2 == "$(SHELLCHECK_VERSION)" { found = 1 } END { exit !found }' || { \
		echo 'ShellCheck $(SHELLCHECK_VERSION) is required' >&2; \
		exit 1; \
	}
	actionlint -pyflakes=
	shellcheck .github/e2e/*.sh tools/*.sh

policy-check:
	sh tools/check-policy.sh

license-check:
	@test -s LICENSE -a -s THIRD_PARTY_NOTICES.md -a -s assets/js/mermaid/LICENSE -a -s assets/fonts/LICENSE.txt || { \
		echo 'a repository or redistributed-asset licence is missing' >&2; \
		exit 1; \
	}
	@set -eu; \
	check_manifest() { \
		directory=$$1; \
		if command -v sha256sum >/dev/null 2>&1; then \
			(cd "$$directory" && sha256sum -c SHA256SUMS); \
		elif command -v shasum >/dev/null 2>&1; then \
			(cd "$$directory" && shasum -a 256 -c SHA256SUMS); \
		else \
			echo 'sha256sum or shasum is required for licence inventories' >&2; \
			exit 1; \
		fi; \
	}; \
	check_manifest assets/js/mermaid; \
	check_manifest assets/fonts
	go mod verify
	go -C tools/sqlite-driver-bakeoff mod verify

# Prepare the exact tagged archive for independent review. This is quarantined
# evidence, not a release: the final report must bind its printed SHA-256 before
# source-artifact may assemble the five-file candidate bundle.
source-archive-candidate:
	@test -n "$(RELEASE_VERSION)" || { echo 'RELEASE_VERSION is required (for example v0.1.0)' >&2; exit 2; }
	@test -n "$(SOURCE_COMMIT)" || { echo 'SOURCE_COMMIT is required as a full 40-character commit' >&2; exit 2; }
	mkdir -p dist/candidates
	@bootstrap=$$(mktemp "$${TMPDIR:-/tmp}/yomihon-source-artifact-bootstrap.XXXXXX"); \
	trap 'rm -f "$$bootstrap"' 0 HUP INT TERM; \
	GIT_NO_REPLACE_OBJECTS=1 git show "$(SOURCE_COMMIT):Makefile" | cmp - Makefile || { echo 'working-tree Makefile differs from SOURCE_COMMIT; use the committed bootstrap command in docs/release.md' >&2; exit 1; }; \
	GIT_NO_REPLACE_OBJECTS=1 git show "$(SOURCE_COMMIT):tools/source-artifact-bootstrap.sh" >"$$bootstrap"; \
	sh "$$bootstrap" --prepare-archive "$(RELEASE_VERSION)" "$(SOURCE_COMMIT)" "$(SOURCE_ARCHIVE)"

# Rebuild the reviewed archive and assemble the source-only candidate bundle
# from an immutable tag. The report's archive digest must match the rebuilt
# bytes. This publishes only to local dist/; public upload is a later release
# action after final bundle inspection.
source-artifact:
	@test -n "$(RELEASE_VERSION)" || { echo 'RELEASE_VERSION is required (for example v0.1.0)' >&2; exit 2; }
	@test -n "$(SOURCE_COMMIT)" || { echo 'SOURCE_COMMIT is required as a full 40-character commit' >&2; exit 2; }
	@test -n "$(REVIEW_EVIDENCE)" || { echo 'REVIEW_EVIDENCE is required as an independent final review record' >&2; exit 2; }
	mkdir -p dist
	@bootstrap=$$(mktemp "$${TMPDIR:-/tmp}/yomihon-source-artifact-bootstrap.XXXXXX"); \
	trap 'rm -f "$$bootstrap"' 0 HUP INT TERM; \
	GIT_NO_REPLACE_OBJECTS=1 git show "$(SOURCE_COMMIT):Makefile" | cmp - Makefile || { echo 'working-tree Makefile differs from SOURCE_COMMIT; use the committed bootstrap command in docs/release.md' >&2; exit 1; }; \
	GIT_NO_REPLACE_OBJECTS=1 git show "$(SOURCE_COMMIT):tools/source-artifact-bootstrap.sh" >"$$bootstrap"; \
	sh "$$bootstrap" --assemble "$(RELEASE_VERSION)" "$(SOURCE_COMMIT)" "$(REVIEW_EVIDENCE)" dist

# Exercise the same builder without requiring a release tag. Two independent
# output directories must be byte-identical. The checksum mutation is expected
# to fail and proves the manifest is an active lock rather than decoration.
source-artifact-check:
	sh tools/test-source-artifact.sh

gen: sqlc
	go tool templ generate -path internal/ui

sqlc: sqlc-version
	sqlc generate
	sqlc vet

sqlc-version:
	@version=$$(sqlc version); [ "$$version" = "$(SQLC_VERSION)" ] || { echo 'sqlc $(SQLC_VERSION) is required, got '"$$version" >&2; exit 1; }

# Generation is intentionally separated from verification. The check generates
# into an empty directory and compares exact trees, so stale output and extra
# hand-written files in the generated package both fail closed.
sqlc-check: sqlc-version
	@set -eu; \
	tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/yomihon-sqlc.XXXXXX"); \
	trap 'rm -rf "$$tmp"' 0 HUP INT TERM; \
	mkdir -p "$$tmp/internal/search/semantic/sql"; \
	cp sqlc.yaml "$$tmp/sqlc.yaml"; \
	cp internal/search/semantic/sql/schema.sql internal/search/semantic/sql/query.sql "$$tmp/internal/search/semantic/sql/"; \
	sqlc generate --no-remote -f "$$tmp/sqlc.yaml"; \
	if ! diff -ru "$$tmp/internal/search/semantic/catalog" internal/search/semantic/catalog; then \
		echo 'sqlc generated tree differs from the checked-in package' >&2; \
		exit 1; \
	fi; \
	sqlc vet

css:
	tailwindcss -i assets/css/input.css -o assets/css/output.css --minify

css-check:
	@help=$$(tailwindcss --help 2>&1); case "$$help" in *"tailwindcss $(TAILWIND_VERSION)"*) ;; *) echo 'tailwindcss $(TAILWIND_VERSION) is required' >&2; exit 1;; esac
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

tools-check-prepare:
	@test "$$(sed -n 's|^replace github.com/koopa0/yomihon => \(.*\)$$|\1|p' tools/sqlite-driver-bakeoff/go.mod)" = "../.." || { \
		echo 'SQLite bake-off replace must point exactly at the root module' >&2; \
		exit 1; \
	}
	go -C tools/sqlite-driver-bakeoff mod tidy -diff

tools-check: tools-check-prepare
	go -C tools/sqlite-driver-bakeoff vet -tags modernc ./...
	@version=$$(golangci-lint version); case "$$version" in *"version $(GOLANGCI_LINT_VERSION) "*) ;; *) echo 'golangci-lint $(GOLANGCI_LINT_VERSION) is required' >&2; exit 1;; esac
	cd tools/sqlite-driver-bakeoff && golangci-lint run --config ../../.golangci.yml --build-tags modernc --disable=gomoddirectives ./...
	@$(call require-go-tool,staticcheck,honnef.co/go/tools,$(STATICCHECK_VERSION))
	cd tools/sqlite-driver-bakeoff && staticcheck -tags modernc ./...
	@$(call require-go-tool,gosec,github.com/securego/gosec/v2,$(GOSEC_VERSION))
	cd tools/sqlite-driver-bakeoff && gosec -tags modernc -tests -nosec-require-rules -nosec-require-justification ./...
	go -C tools/sqlite-driver-bakeoff test -race -tags modernc ./...

# The rejected CGO driver remains only to reproduce the documented comparison.
# It is not part of the product gate because requiring a C toolchain would undo
# the selected driver's portability advantage. CI calls this target on Linux so
# retained comparison code cannot rot unnoticed.
tools-check-mattn: tools-check-prepare
	go -C tools/sqlite-driver-bakeoff vet -tags mattn ./...
	@version=$$(golangci-lint version); case "$$version" in *"version $(GOLANGCI_LINT_VERSION) "*) ;; *) echo 'golangci-lint $(GOLANGCI_LINT_VERSION) is required' >&2; exit 1;; esac
	cd tools/sqlite-driver-bakeoff && golangci-lint run --config ../../.golangci.yml --build-tags mattn --disable=gomoddirectives ./...
	@$(call require-go-tool,staticcheck,honnef.co/go/tools,$(STATICCHECK_VERSION))
	cd tools/sqlite-driver-bakeoff && staticcheck -tags mattn ./...
	@$(call require-go-tool,gosec,github.com/securego/gosec/v2,$(GOSEC_VERSION))
	cd tools/sqlite-driver-bakeoff && gosec -tags mattn -tests -nosec-require-rules -nosec-require-justification ./...
	go -C tools/sqlite-driver-bakeoff test -race -tags mattn ./...

frontend-check:
	npm ci --prefix .github --ignore-scripts --no-audit --fund=false
	npm exec --prefix .github -- biome lint --error-on-warnings assets/js/*.js .github/e2e/*.mjs .github/*.mjs
	@$(MAKE) --no-print-directory stylelint-check

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
	bash .github/e2e/serve.sh "$$tmp/yomihon" 19734 -- bash .github/e2e/probes.sh; \
	bash .github/e2e/serve.sh "$$tmp/yomihon" 19736 -- node .github/status-activation-contract.mjs

mutation-check: frontend-check
	@set -eu; \
	tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/yomihon-mutations.XXXXXX"); \
	trap 'rm -rf "$$tmp"' 0 HUP INT TERM; \
	go build -o "$$tmp/yomihon" ./cmd/yomihon; \
	bash .github/e2e/serve.sh "$$tmp/yomihon" 19735 -- bash .github/e2e/probes.sh --mutate; \
	bash .github/e2e/serve.sh "$$tmp/yomihon" 19737 -- make --no-print-directory status-activation-mutations

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

# The standalone status activation contract is wired directly by CI. Keep the
# same mutation proof as the browser-probe registry: each listed mode must exit
# 1 for its named assertion, never merely non-zero because its rewrite failed.
status-activation-mutations:
	@set -eu; \
	modes=$$(MUTATE=list node .github/status-activation-contract.mjs); \
	[ -n "$$modes" ] || { echo 'status activation contract names no mutation modes' >&2; exit 1; }; \
	for mode in $$modes; do \
		status=0; \
		out=$$(MUTATE="$$mode" node .github/status-activation-contract.mjs 2>&1) || status=$$?; \
		printf '%s\n' "$$out"; \
		[ "$$status" -eq 1 ] || { echo "status activation MUTATE=$$mode exited $$status, want 1" >&2; exit 1; }; \
		printf '%s\n' "$$out" | grep -qxF "MUTATE-RESULT: caught $$mode" || { \
			echo "status activation MUTATE=$$mode exited 1 without its exact caught marker" >&2; \
			exit 1; \
		}; \
	done

verify: policy-check mod-check fmt-check css-check vet lint staticcheck gosec vuln test real-vault-build-check tools-check workflow-check build-check frontend-check e2e-http-check fuzz-smoke browser-check mutation-check portable-build-check performance-smoke license-check source-artifact-check

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
