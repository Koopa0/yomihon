#!/usr/bin/env bash
# Idempotent Cloud Agent bootstrap for yomihon. Prepares the toolchain and
# generated state a fresh checkout needs to build and serve the reader:
#
#   - the pinned Tailwind standalone CLI (matches Makefile / CI),
#   - the Go module cache (which also fetches the go.mod templ tool),
#   - the Node dev tools the e2e browser probes drive,
#   - the committed generated sources (templ output and the stylesheet), and
#   - a compiled server binary so the terminal can serve immediately.
#
# It touches nothing under version control except regenerating already-committed
# generated files, so a second run converges without changes.
set -euo pipefail

# Kept in step with Makefile TAILWIND_VERSION and the CI pin. The checksum is
# the Linux x86-64 artifact published with that release.
TAILWIND_VERSION="v4.1.17"
TAILWIND_SHA256="cc115d9b6c4ede4e423bfea6a3cfc3f03e6c1702b7d910369b9540e2b4cf3860"

# tailwind_current reports whether the pinned CLI is already on PATH, so a
# re-run neither re-downloads nor needs elevated privileges.
tailwind_current() {
	command -v tailwindcss >/dev/null 2>&1 || return 1
	case "$(NO_COLOR=1 tailwindcss --help 2>&1)" in
	*"tailwindcss ${TAILWIND_VERSION}"*) return 0 ;;
	*) return 1 ;;
	esac
}

if tailwind_current; then
	echo "tailwindcss ${TAILWIND_VERSION} already installed; skipping download"
else
	echo "installing tailwindcss ${TAILWIND_VERSION}"
	tmp="$(mktemp)"
	trap 'rm -f "$tmp"' EXIT
	curl -sSfL \
		"https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWIND_VERSION}/tailwindcss-linux-x64" \
		-o "$tmp"
	echo "${TAILWIND_SHA256}  ${tmp}" | sha256sum -c -
	chmod +x "$tmp"
	sudo install -m 0755 "$tmp" /usr/local/bin/tailwindcss
	rm -f "$tmp"
	trap - EXIT
fi

# Populate the module cache and materialise the go.mod tool (templ) so the
# build and generation steps below run offline-fast and deterministically.
go mod download
go tool templ --version >/dev/null

# Frontend linters and the Playwright driver the e2e browser probes use. The
# product build needs none of this; it lives only under .github/.
npm ci --prefix .github --ignore-scripts --no-audit --fund=false

# Regenerate the committed generated sources the Go build and CSS depend on. A
# clean tree leaves these unchanged; a stale tree is repaired before the build.
go tool templ generate -path internal/ui
tailwindcss -i assets/css/input.css -o assets/css/output.css --minify

# Compile the server so a booted environment can serve without a cold build.
go build -o bin/yomihon ./cmd/yomihon

echo "yomihon environment ready"
