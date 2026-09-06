NODE_BIN := node_modules/.bin
ESBUILD := $(NODE_BIN)/esbuild
OVSX = npx --no-install ovsx
PUBLISHER = arandu-io
NAME = arandu
VSCE := $(NODE_BIN)/vsce
GO := GOWORK=off go
VERSION := $(shell $(GO) run ./cmd/manifest-version package.json)
VSIX := dist/arandu-$(VERSION).vsix
BUNDLE := dist/extension.js
BUNDLE_FIRST := dist/.extension-first.js
BUNDLE_SECOND := dist/.extension-second.js
FIRST := dist/.arandu-first.vsix
SECOND := dist/.arandu-second.vsix
RAW_FIRST := dist/.arandu-first.raw.vsix
RAW_SECOND := dist/.arandu-second.raw.vsix

.PHONY: audit bundle check format-check json-contracts package publish-check publish-openvsx test typecheck vet

check: format-check vet test audit typecheck package

format-check:
	@unformatted="$$(find . -name '*.go' -not -path './node_modules/*' -not -path './dist/*' -not -path '*/testdata/*' -exec gofmt -l {} +)"; \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt'd:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

vet:
	$(GO) vet ./...

test:
	$(GO) test -race -count=1 ./...

json-contracts:
	$(GO) test -race -count=1 ./tests/Feature/extension \
		-run '^(TestTheExtension|TestTheGrammar|TestTheEditor|TestTheProjectMap|TestDoctor|TestSnippets)'

audit:
	$(GO) run ./cmd/repository-audit .

typecheck:
	$(NODE_BIN)/tsc --noEmit

bundle:
	mkdir -p dist
	$(ESBUILD) src/extension.ts --bundle --platform=node --format=cjs --target=node20 --external:vscode --legal-comments=eof --outfile=$(BUNDLE_FIRST)
	$(ESBUILD) src/extension.ts --bundle --platform=node --format=cjs --target=node20 --external:vscode --legal-comments=eof --outfile=$(BUNDLE_SECOND)
	cmp $(BUNDLE_FIRST) $(BUNDLE_SECOND)
	mv $(BUNDLE_FIRST) $(BUNDLE)
	rm -f $(BUNDLE_SECOND)

package: bundle
	$(VSCE) package --out $(RAW_FIRST)
	$(GO) run ./cmd/vsix-repack $(RAW_FIRST) $(FIRST)
	$(VSCE) package --out $(RAW_SECOND)
	$(GO) run ./cmd/vsix-repack $(RAW_SECOND) $(SECOND)
	cmp $(FIRST) $(SECOND)
	mv $(FIRST) $(VSIX)
	rm -f $(SECOND) $(RAW_FIRST) $(RAW_SECOND)
	$(GO) run ./cmd/vsix-audit $(VSIX)

# Publish the audited package to Open VSX, which is the registry the editors
# built on VS Code read: Cursor, Windsurf, Antigravity, VSCodium and Theia.
# Microsoft's own marketplace refuses them by its terms of service, so this is
# the only registry that reaches those editors at all.
#
# It builds the package first, so what is published is what the audit passed --
# never a file left in dist from an earlier build.
#
# OVSX_PAT comes from open-vsx.org, from an account that has signed the Eclipse
# Foundation Publisher Agreement. Without that signature the upload is refused
# with a message about the agreement, not about the token.
publish-openvsx: package
	@test -n "$(OVSX_PAT)" || { echo "OVSX_PAT is unset: publishing needs a token from open-vsx.org"; exit 1; }
	$(OVSX) publish $(VSIX) --pat $(OVSX_PAT)

# What the publish would do, without doing it. It answers the one question a
# dry run is for: is this identifier free, and does the file the audit passed
# look the way the registry expects.
publish-check: package
	$(OVSX) get $(PUBLISHER).$(NAME) --metadata 2>&1 | head -20 || true
	@echo "package ready: $(VSIX)"
