# Contributing

## Sign your commits

Every commit needs a `Signed-off-by` line:

```
git commit -s -m "what changed and why"
```

That line is the [Developer Certificate of Origin](https://developercertificate.org/):
you are stating that you wrote the change, or that you have the right to submit
it under this project's license. It is not a copyright assignment — you keep
your copyright, and this project can never be relicensed behind your back.

We use DCO rather than a CLA on purpose. A CLA would let the project relicense
later, and the price is that every contributor has to sign a legal document
before their first patch.

## Before you open a pull request

```
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*')
go vet ./...
go test -race -count=1 ./...
go run ./cmd/repository-audit .
make package
```

The extension itself is declarative. Go owns its contracts and package
auditors; the pinned VSCE invocation only turns the approved assets into the
platform's required VSIX envelope. `make package` builds twice, compares both
archives byte for byte, and checks their contents against the Go allowlist. CI
also blocks known vulnerabilities with `govulncheck`.

## Where a test goes

User-visible extension contracts live under `tests/Feature/extension/` and
exercise the published JSON and command seams. An implementation test that
genuinely needs an unexported Go identifier stays beside that command and uses
the `_internal_test.go` suffix.

## What the commit message says

What changed and why. The why is the part that is not in the diff, and it is the
part someone will need in two years.

No AI attribution of any kind: no `Co-Authored-By` for an assistant, no
"generated with" footer. Commits are authored by the people who submit them.

## Architecture decisions

The decisions this project has already made live in `arandu-io/docs`, and every
one that closed a door has an ADR. If your change contradicts one, say so in the
pull request and argue for the change of decision — that is a normal thing to
do, and it is better than a patch that quietly works around it.
