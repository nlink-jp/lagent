BINARY  := lagent
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
DIST_DIR := dist

# macOS Developer ID signing / notarization (see nlink-jp/.github
# CONVENTIONS.md §Code Signing). Defaults match any Developer ID
# Application cert in the keychain and the org-standard notary
# profile. Builds without these fall back to ad-hoc / un-notarized
# with a one-line warning — see scripts/codesign-darwin.sh.
CODESIGN_IDENTITY ?= Developer ID Application
NOTARY_PROFILE    ?= nlink-jp-notary

.PHONY: build build-all package verify-release test vet lint docs-check gate-check check clean

build:
	@mkdir -p $(DIST_DIR)
	go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY) .
	@scripts/codesign-darwin.sh $(DIST_DIR)/$(BINARY) "$(CODESIGN_IDENTITY)"

# lagent is macOS-only by design (sandbox-exec based isolation, as
# gem-agent); darwin ships arm64 only per the org Release Archive Standard.
build-all:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-arm64 .
	@scripts/codesign-darwin.sh $(DIST_DIR)/$(BINARY)-darwin-arm64 "$(CODESIGN_IDENTITY)" "$(BINARY)"

## package: Archive the darwin build with the canonical binary name +
## README.md + LICENSE, then notarize. Asset naming follows the org
## Release Archive Standard (lagent-vX.Y.Z-darwin-arm64.zip).
package: build-all
	@cd $(DIST_DIR) && \
		stage=_pkg; rm -rf $$stage; mkdir -p $$stage; \
		cp "$(BINARY)-darwin-arm64" "$$stage/$(BINARY)"; \
		cp ../README.md ../LICENSE $$stage/; \
		( cd $$stage && zip -q "../$(BINARY)-$(VERSION)-darwin-arm64.zip" * ); \
		rm -rf $$stage
	@scripts/notarize-darwin.sh $(DIST_DIR)/$(BINARY)-$(VERSION)-darwin-arm64.zip "$(NOTARY_PROFILE)"

# verify-release inspects the packaged zip from INSIDE the repo. The
# notarization marker is the gate, written by notarize-darwin.sh only on
# "status: Accepted". Unpacking, running, and reporting this version are
# hard gates, each with its own failure line; the spctl probe is
# informational only. What the gate does on a bad zip is itself checked
# by scripts/verify-release-selftest.sh (run by `make check`).
verify-release:
	@test -f "$(DIST_DIR)/$(BINARY)-$(VERSION)-darwin-arm64.zip.notarized" || { \
		echo "verify-release: FAIL — $(BINARY)-$(VERSION)-darwin-arm64.zip has no notarization marker."; \
		echo "  make package must end with '[notarize] ...: Accepted'. Do not upload this zip."; \
		exit 1; }
	@test "$(DIST_DIR)/$(BINARY)-$(VERSION)-darwin-arm64.zip.notarized" -nt "$(DIST_DIR)/$(BINARY)-$(VERSION)-darwin-arm64.zip" || { \
		echo "verify-release: FAIL — the zip was rebuilt after its marker (re-run make package)."; \
		exit 1; }
	@tmp=$$(mktemp -d); rc=0; \
		if ! unzip -oq "$(DIST_DIR)/$(BINARY)-$(VERSION)-darwin-arm64.zip" -d "$$tmp"; then \
			echo "verify-release: FAIL — the zip does not unpack. Do not upload it."; rc=1; \
		elif ! out=$$("$$tmp/$(BINARY)" --version 2>&1); then \
			echo "verify-release: FAIL — the packaged binary does not run:"; \
			echo "  $$out"; rc=1; \
		elif ! printf '%s\n' "$$out" | grep -qF "$(VERSION)"; then \
			echo "verify-release: FAIL — the packaged binary reports \"$$out\", not $(VERSION)."; \
			echo "  The zip holds a build from another tag (re-run make package)."; rc=1; \
		else \
			echo "  $$out"; \
			spctl -a -vv -t install "$$tmp/$(BINARY)" 2>&1 | head -2 || true; \
		fi; \
		rm -rf "$$tmp"; \
		exit $$rc
	@echo "verify-release: OK ($(VERSION), notarized, unpacks, runs, reports its version)"

test:
	go test ./...

vet:
	go vet ./...

## lint: golangci-lint with the org config (.golangci.yml). errcheck is
## on everywhere except writes to the CLI's own streams.
lint:
	golangci-lint run ./...

## docs-check: docs/en and docs/ja must be full structural mirrors, the
## ADR catalogue complete, and identifiers agree across each pair.
docs-check:
	@scripts/docs-mirror-check.sh

## gate-check: verify-release must reject a bad zip.
gate-check:
	@scripts/verify-release-selftest.sh

check: vet lint test docs-check gate-check build

clean:
	rm -rf $(DIST_DIR)

# Homebrew tap generation (see scripts/release-brew.mk). After `make package`,
# `make brew` generates this formula from the built darwin-arm64 zip into the
# local nlink-jp/homebrew-tap checkout. The package target is unchanged.
BREW_KIND := formula
BREW_DESC := Sandboxed coding-agent runtime on a local LLM (LM Studio / Ollama)
include scripts/release-brew.mk
