NODE_BIN := node_modules/.bin
ESBUILD := $(NODE_BIN)/esbuild
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

.PHONY: audit bundle check format-check json-contracts package test typecheck vet

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
