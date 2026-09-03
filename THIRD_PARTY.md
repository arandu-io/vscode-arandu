# Third-party notices

Arandu Intelligence is MIT licensed (see `LICENSE.md`). This file covers the
third-party work that is **bundled** into `dist/extension.js` and therefore
redistributed inside every `.vsix` this repository publishes — including copies
installed by people who never ran `npm install` and never saw a `node_modules`
directory.

That is what makes this file necessary rather than polite. The extension is
built with

```
esbuild src/extension.ts --bundle --platform=node --format=cjs --target=node20 \
  --external:vscode --legal-comments=eof --outfile=dist/extension.js
```

Only `vscode` is external — the editor supplies it at runtime. Everything else
resolved from `node_modules/` is inlined into the single file that ships, and
the published `.vsix` carries no `node_modules/` to point at instead. Every
installation of this extension is therefore a redistribution of nine packages,
and a redistributor of MIT, ISC and Blue Oak licensed code owes the notice.

`--legal-comments=eof` does not discharge that obligation here, and the reason
is worth writing down rather than assuming. esbuild only preserves a comment it
can recognise as legal: one beginning `//!` or `/*!`, or containing `@license`
or `@preserve`. None of the nine packages below writes its copyright that way —
their notices live in a separate `License.txt`, `LICENSE` or `LICENSE.md` file
that the bundler never reads — so `grep -c Copyright dist/extension.js` returns
`0`. The flag is kept because it costs nothing and would carry a notice the day
a dependency starts emitting one; it is not the thing that satisfies the
licenses. This file is.

The nine are not nine direct dependencies. `package.json` declares exactly one,
`vscode-languageclient`; the other eight arrive through it. A transitive
dependency is redistributed on exactly the same terms as a direct one, so the
list below is derived from what is actually in the bundle rather than from what
the manifest asks for.

Keeping it current is checked, not remembered:
`TestEveryBundledDependencyIsCredited` in
`tests/Feature/extension/third_party_test.go` reads the
`// node_modules/<package>/` markers esbuild writes into `dist/extension.js` and
fails when one of them has no entry here, when the version credited here stops
matching the version resolved in `package-lock.json`, or when a license is named
without its text.

---

## vscode-languageclient — `extension/dist/extension.js`

| | |
|---|---|
| Version | 10.1.1 |
| Author | Microsoft Corporation |
| Home | https://github.com/microsoft/vscode-languageserver-node |
| License | MIT |

The only dependency this extension declares, and the largest thing in the
bundle: 57 of its CommonJS modules are inlined. It is the client half of the
Language Server Protocol — it spawns `aru`, speaks LSP to it over stdio, and
maps the results onto the editor's diagnostics, completion, hover and document
symbol APIs. The extension imports `LanguageClient`, `TransportKind` and the
options types from it; the rest of the 57 modules are the feature registrations
that the client pulls in for itself.

```
Copyright (c) Microsoft Corporation

All rights reserved.

MIT License

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED *AS IS*, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

---

## vscode-languageserver-protocol — `extension/dist/extension.js`

| | |
|---|---|
| Version | 3.18.3 |
| Author | Microsoft Corporation |
| Home | https://github.com/microsoft/vscode-languageserver-node |
| License | MIT |

Reached through `vscode-languageclient`, never imported by Arandu code, and 28
of its modules are inlined. It is the protocol layer proper: the request and
notification type declarations for every LSP method, and the capability
structures the client and the server exchange during initialisation.

```
Copyright (c) Microsoft Corporation

All rights reserved.

MIT License

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED *AS IS*, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

---

## vscode-jsonrpc — `extension/dist/extension.js`

| | |
|---|---|
| Version | 9.0.2 |
| Author | Microsoft Corporation |
| Home | https://github.com/microsoft/vscode-languageserver-node |
| License | MIT |

Reached through `vscode-languageserver-protocol`, and 16 of its modules are
inlined. It is the transport underneath the protocol: JSON-RPC framing, the
message reader and writer over a stream, cancellation tokens, and the
`node`-flavoured connection that binds them to the child process running `aru`.

```
Copyright (c) Microsoft Corporation

All rights reserved.

MIT License

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED *AS IS*, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

---

## vscode-languageserver-types — `extension/dist/extension.js`

| | |
|---|---|
| Version | 3.18.3 |
| Author | Microsoft Corporation |
| Home | https://github.com/microsoft/vscode-languageserver-node |
| License | MIT |

Reached through `vscode-languageserver-protocol`. One module is inlined, and it
is small: the runtime constructors and type guards for the data shapes LSP
passes around — `Position`, `Range`, `Location`, `Diagnostic`, `TextEdit`,
`SymbolKind` and their neighbours. Most of the package is TypeScript
declarations, which the bundler erases; what ships is the handful of values
those declarations describe.

```
Copyright (c) Microsoft Corporation

All rights reserved.

MIT License

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED *AS IS*, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

---

## vscode-languageserver-textdocument — `extension/dist/extension.js`

| | |
|---|---|
| Version | 1.0.14 |
| Author | Microsoft Corporation |
| Home | https://github.com/microsoft/vscode-languageserver-node |
| License | MIT |

Reached through `vscode-languageclient`, with one module inlined. It is the
in-memory text document: it keeps a document's content and version, applies
incremental content changes, and converts between offsets and line/character
positions so the client can talk about ranges the same way the server does.

```
Copyright (c) Microsoft Corporation

All rights reserved.

MIT License

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED *AS IS*, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

---

## semver — `extension/dist/extension.js`

| | |
|---|---|
| Version | 7.8.5 |
| Author | Isaac Z. Schlueter and Contributors (maintained by GitHub, Inc.) |
| Home | https://github.com/npm/node-semver |
| License | ISC |

Reached through `vscode-languageclient`, with 19 of its modules inlined: the
`SemVer` class, the range and comparator classes, and the parsing internals they
share. The client uses it to compare the running editor's version against the
minimum an LSP feature requires before registering that feature.

The author line above records the copyright holder named in the license file.
The package is published by the npm team at GitHub, Inc., which is what
`package.json` names; the notice follows the license, because that is the text
with the obligation attached to it.

ISC is the same bargain as MIT written shorter: keep the copyright notice and
the permission notice with the copies you hand on.

```
The ISC License

Copyright (c) Isaac Z. Schlueter and Contributors

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR
IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
```

---

## minimatch — `extension/dist/extension.js`

| | |
|---|---|
| Version | 10.2.6 |
| Author | Isaac Z. Schlueter |
| Home | https://github.com/isaacs/minimatch |
| License | Blue Oak Model License 1.0.0 (BlueOak-1.0.0) |

Reached through `vscode-languageclient`, with 6 of its modules inlined: the
matcher itself plus its brace-expression, AST, escape and unescape helpers. The
client uses it to decide whether a document matches a `DocumentSelector`
pattern — that is, whether a file the editor just opened belongs to the language
server at all.

**This is the one entry here whose license is not MIT, and it is the reason this
file has to exist rather than merely being good practice.** The Blue Oak Model
License carries an explicit *Notices* section: everyone who receives any part of
the software from us, changed or unchanged, must also receive the text of the
license or a link to <https://blueoakcouncil.org/license/1.0.0>. Publishing a
`.vsix` with `minimatch` inlined and no notice anywhere in the payload is a
breach of that clause. The license then grants 30 days from written notice to
put it right, after which it ends immediately — so this section, shipped inside
the `.vsix` as `extension/THIRD_PARTY.md`, is the compliance, not the paperwork
about it.

```
# Blue Oak Model License

Version 1.0.0

## Purpose

This license gives everyone as much permission to work with
this software as possible, while protecting contributors
from liability.

## Acceptance

In order to receive this license, you must agree to its
rules. The rules of this license are both obligations
under that agreement and conditions to your license.
You must not do anything with this software that triggers
a rule that you cannot or will not follow.

## Copyright

Each contributor licenses you to do everything with this
software that would otherwise infringe that contributor's
copyright in it.

## Notices

You must ensure that everyone who gets a copy of
any part of this software from you, with or without
changes, also gets the text of this license or a link to
<https://blueoakcouncil.org/license/1.0.0>.

## Excuse

If anyone notifies you in writing that you have not
complied with [Notices](#notices), you can keep your
license by taking all practical steps to comply within 30
days after the notice. If you do not do so, your license
ends immediately.

## Patent

Each contributor licenses you to do everything with this
software that would otherwise infringe any patent claims
they can license or become able to license.

## Reliability

No contributor can revoke this license.

## No Liability

**_As far as the law allows, this software comes as is,
without any warranty or condition, and no contributor
will be liable to anyone for any damages related to this
software or this license, under any kind of legal claim._**
```

---

## brace-expansion — `extension/dist/extension.js`

| | |
|---|---|
| Version | 5.0.9 |
| Author | Julian Gruber; TypeScript port by Isaac Z. Schlueter |
| Home | https://github.com/juliangruber/brace-expansion |
| License | MIT |

Reached through `minimatch`, with one module inlined. It expands the brace
sections of a glob — `{ts,go}` into two alternatives, `{1..3}` into a
sequence — before the matcher compiles the rest of the pattern.

Its license names two copyright holders, the original author and the author of
the TypeScript port, and both are reproduced below because MIT requires the
notice as written and not a summary of it.

```
MIT License

Copyright Julian Gruber <julian@juliangruber.com>

TypeScript port Copyright Isaac Z. Schlueter <i@izs.me>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

---

## balanced-match — `extension/dist/extension.js`

| | |
|---|---|
| Version | 4.0.4 |
| Author | Julian Gruber; TypeScript port by Isaac Z. Schlueter |
| Home | https://github.com/juliangruber/balanced-match |
| License | MIT |

Reached through `brace-expansion`, with one module inlined, and the smallest
thing in the bundle. It finds the matching pair of delimiters in a string, which
is how `brace-expansion` locates the boundaries of the brace section it is about
to expand.

Its license, like `brace-expansion`'s, names both the original author and the
author of the TypeScript port.

```
(MIT)

Original code Copyright Julian Gruber <julian@juliangruber.com>

Port to TypeScript Copyright Isaac Z. Schlueter <i@izs.me>

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
of the Software, and to permit persons to whom the Software is furnished to do
so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

---

## Arandu's own files

Listed so that every entry in the published `.vsix` is accounted for, and so
that adding one is a decision somebody wrote down rather than an omission. The
list matches the allowlist in `cmd/vsix-audit/main.go`, which fails the build if
the archive gains a file that is not here or loses one that is.

- `extension/dist/extension.js` — the Arandu half of it: everything compiled
  from `src/`, which is the activation entry point, the Aru discovery and
  version checks, the language client wiring, the Project Map and Development
  tree views, and the development-server process control. It is Arandu, MIT,
  covered by `LICENSE.md`. The nine packages credited above are the other half
  of the same file.
- `extension/package.json` — Arandu, MIT, covered by `LICENSE.md`. The extension
  manifest: commands, views, configuration, the `kyse` language contribution.
- `extension/LICENSE.md` — Arandu's own license, MIT.
- `extension/THIRD_PARTY.md` — this file.
- `extension/readme.md` and `extension/changelog.md` — Arandu, MIT, covered by
  `LICENSE.md`. Renamed in the archive by the packaging tool; they are `README.md`
  and `CHANGELOG.md` in the repository.
- `extension/images/aru.svg`, `extension/images/icon.png`,
  `extension/images/logo.png` — Arandu's own marks, MIT, covered by
  `LICENSE.md`. Drawn for this project; they contain no third-party artwork and
  no icon-set glyph.
- `extension/language-configuration.json` — Arandu, MIT, covered by
  `LICENSE.md`. Brackets, comments and auto-closing pairs for `.kyse.go`.
- `extension/syntaxes/kyse.tmLanguage.json` — Arandu, MIT, covered by
  `LICENSE.md`. The Kyse grammar, written here. It embeds `source.go` and
  `text.html.basic` by scope name, which is a reference to grammars the editor
  already ships, not a copy of them.
- `extension/snippets/kyse.json` — Arandu, MIT, covered by `LICENSE.md`.
- `[Content_Types].xml` and `extension.vsixmanifest` — neither Arandu's nor a
  dependency's: they are generated at package time by `@vscode/vsce`, which is a
  build tool and is not itself redistributed. They contain no third-party code.
