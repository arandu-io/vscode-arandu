<p align="center">
  <img src="images/logo.png" alt="Arandu" width="180">
</p>

<h1 align="center">Arandu for Visual Studio Code</h1>

<p align="center">First-party Kyse language intelligence and project navigation for Arandu.</p>

<p align="center">
<a href="https://github.com/arandu-io/vscode-arandu/actions/workflows/ci.yml"><img src="https://github.com/arandu-io/vscode-arandu/actions/workflows/ci.yml/badge.svg" alt="Build Status"></a>
<a href="https://github.com/arandu-io/vscode-arandu/releases"><img src="https://img.shields.io/github/v/release/arandu-io/vscode-arandu?label=version" alt="Latest Version"></a>
<a href="LICENSE.md"><img src="https://img.shields.io/github/license/arandu-io/vscode-arandu" alt="License"></a>
</p>

## Installation

Install the current Aru CLI first:

```bash
brew install arandu-io/tap/aru
```

The release process attaches a reproducible `arandu-*.vsix` to every
[GitHub release](https://github.com/arandu-io/vscode-arandu/releases). Download
it and install it from VS Code with **Extensions: Install from VSIX**, or run:

```bash
code --install-extension arandu-<version>.vsix
```

## Preview

The extension owns `.kyse.go` files and supplies Kyse syntax highlighting,
comment and indentation rules, and snippets for complete views, layouts,
control flow, composition, component attributes, CSRF, and both interpolation
forms. Highlighting continues inside an HTML opening tag: a directive on its
own line between attributes, such as `@if(...)` or `@attributes(...)`, and a
`{{ }}` or `{!! !!}` inside an attribute value are colored as Kyse rather than
as HTML. Its language client connects to `aru lsp` for completion,
diagnostics, and go-to-definition from a view to the layouts and components it
names and from Go source to the views it names.

The Arandu activity container has two native views. Project Map starts with the
active-project selector, then shows application features, HTTP, database,
views, async, console, native screens, native capabilities, community modules,
and diagnostics; located items open at their source line. Development exposes
visible actions to select the project, start, stop, or restart `aru dev`, run
Doctor immediately, and configure the Aru executable.

When the project has a native target, the Project Map toolbar also offers
`Arandu: Run Native Application`, and the command palette adds
`Arandu: Build Native Application` and `Arandu: Watch Native Application`. Each
runs the matching `aru native:*` command in a terminal, and Run and Watch keep a
single native window open.

Doctor findings for the selected project also appear in VS Code Problems, with
stale findings cleared on every refresh. Doctor runs when the extension starts
and, with debounce, after a change to any Go file in that project or to another
file Doctor reads, such as `arandu.toml`, `go.mod`, or `.env.example`. Changes
under `vendor`, `node_modules`, `testdata`, `bin`, `.git`, and the views `aru`
writes to `storage/framework/views` do not trigger it.

The map also refreshes from its toolbar. `Arandu: Start Development Server`,
`Stop`, and `Restart` run `aru dev` in a dedicated terminal only after an
explicit command; the extension never runs migrations, seeders, or generators.

## Aru discovery

Open a trusted local workspace containing one or more `arandu.toml` files. The
extension discovers projects nested below every filesystem workspace folder. A
single project is selected automatically; when several exist, choose one from
the Project Map row or either view toolbar. The choice is remembered for that
workspace and becomes the single root for the language server, Project Map,
Doctor, file watcher, Development terminal, and visible Homebrew task.

For the selected project, the adapter resolves `aru` in this order:

1. The workspace setting `arandu.aru.path`.
2. `PATH`.
3. `/opt/homebrew/bin/aru` (Apple Silicon Homebrew).
4. `/usr/local/bin/aru` (Intel Homebrew).

Use `Arandu: Configure Aru Path` when the executable lives elsewhere. The
status bar reports language-server startup, readiness, failures, and whether
the development server is running. Untrusted or virtual workspaces never
start an Aru process.

The extension checks the official stable Aru release at most once every 24
hours. When the installed CLI is older, the status bar and one version-specific
warning offer **Update with Homebrew**. Choosing it runs `brew upgrade
arandu-io/tap/aru` as a visible VS Code task; dismissing the warning suppresses
it for that release, and the extension never updates the CLI automatically.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). CI runs the same Go contracts and VSIX
allowlist documented there.

## Security Vulnerabilities

Please review [our security policy](SECURITY.md). Never open a public issue for
a vulnerability.

## License

Open-sourced software licensed under the [MIT license](LICENSE.md).
