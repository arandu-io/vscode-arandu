VSCE_VERSION := 3.9.2
VERSION := $(shell GOWORK=off go run ./cmd/manifest-version package.json)
VSIX := dist/arandu-$(VERSION).vsix
FIRST := dist/.arandu-first.vsix
SECOND := dist/.arandu-second.vsix
RAW_FIRST := dist/.arandu-first.raw.vsix
RAW_SECOND := dist/.arandu-second.raw.vsix

.PHONY: audit check format-check json-contracts package test vet

check: format-check vet test audit package

format-check:
	@unformatted="$$(find . -name '*.go' -not -path '*/testdata/*' -exec gofmt -l {} +)"; \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt'd:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

vet:
	go vet ./...

test:
	go test -race -count=1 ./...

json-contracts:
	go test -race -count=1 ./tests/Feature/extension \
		-run '^(TestTheExtension|TestTheGrammar|TestTheEditor|TestSnippets)'

audit:
	go run ./cmd/repository-audit .

package:
	mkdir -p dist
	npx --yes @vscode/vsce@$(VSCE_VERSION) package --out $(RAW_FIRST)
	go run ./cmd/vsix-repack $(RAW_FIRST) $(FIRST)
	npx --yes @vscode/vsce@$(VSCE_VERSION) package --out $(RAW_SECOND)
	go run ./cmd/vsix-repack $(RAW_SECOND) $(SECOND)
	cmp $(FIRST) $(SECOND)
	mv $(FIRST) $(VSIX)
	rm -f $(SECOND) $(RAW_FIRST) $(RAW_SECOND)
	go run ./cmd/vsix-audit $(VSIX)
