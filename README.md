<p align="center">
  <img src="images/logo.png" alt="Arandu Intelligence" width="140" height="140">
</p>

<h1 align="center">Arandu Intelligence for Visual Studio Code</h1>

<p align="center">First-party language intelligence, project navigation, and development tools for Arandu.</p>

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
code --install-extension arandu-0.1.0.vsix
```

## Preview

The extension owns `.kyse.go` files and supplies Kyse syntax highlighting,
comment and indentation rules, and snippets for complete views, layouts,
control flow, composition, CSRF, and both interpolation forms. Its language
client connects to `aru lsp` for completion and diagnostics.

The Arandu activity container has two native views. Project Map starts with the
active-project selector, then shows application features, HTTP, database,
views, async, console, native capabilities, community modules, and diagnostics;
located items open at their source line. Development exposes visible actions to
select the project, start, stop, or restart `aru dev`, run Doctor immediately,
and configure the Aru executable.

Doctor findings for the selected project also appear in VS Code Problems, with
stale findings cleared on every refresh. Doctor runs when the extension starts
and, with debounce, after relevant files in that project are saved.

The map also refreshes from its toolbar. `Arandu Intelligence: Start Development Server`,
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

Use `Arandu Intelligence: Configure Aru Path` when the executable lives elsewhere. The
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
