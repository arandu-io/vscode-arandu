<p align="center">
  <img src="images/icon.png" alt="Arandu" width="140" height="140">
</p>

<h1 align="center">Arandu for Visual Studio Code</h1>

<p align="center">First-party Kyse language intelligence and project navigation for Arandu.</p>

<p align="center">
<a href="https://github.com/arandu-io/vscode-arandu/actions/workflows/ci.yml"><img src="https://github.com/arandu-io/vscode-arandu/actions/workflows/ci.yml/badge.svg" alt="Build Status"></a>
<a href="https://github.com/arandu-io/vscode-arandu/releases"><img src="https://img.shields.io/github/v/release/arandu-io/vscode-arandu?label=version" alt="Latest Version"></a>
<a href="LICENSE.md"><img src="https://img.shields.io/github/license/arandu-io/vscode-arandu" alt="License"></a>
</p>

## Preview

The extension owns `.kyse.go` files and supplies Kyse syntax highlighting,
comment and indentation rules, and snippets for complete views, layouts,
control flow, composition, CSRF, and both interpolation forms. Its language
client connects to `aru lsp` for completion and diagnostics.

The Arandu activity container has two native views. Project Map shows
application features, HTTP, database, views, async, console, native
capabilities, community modules, and diagnostics; located items open at their
source line. Development exposes visible actions to start, stop, or restart
`aru dev`, run Doctor immediately, and configure the Aru executable.

Doctor findings also appear in VS Code Problems, with stale findings cleared
on every refresh. Doctor runs when the extension starts and, with debounce,
after relevant project files are saved.

The map also refreshes from its toolbar. `Arandu: Start Development Server`,
`Stop`, and `Restart` run `aru dev` in a dedicated terminal only after an
explicit command; the extension never runs migrations, seeders, or generators.

## Aru discovery

Open a trusted local workspace whose root contains `arandu.toml`. The adapter
resolves `aru` in this order:

1. The workspace setting `arandu.aru.path`.
2. `PATH`.
3. `/opt/homebrew/bin/aru` (Apple Silicon Homebrew).
4. `/usr/local/bin/aru` (Intel Homebrew).

Use `Arandu: Configure Aru Path` when the executable lives elsewhere. The
status bar reports language-server startup, readiness, failures, and whether
the development server is running. Untrusted or virtual workspaces never
start an Aru process.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). CI runs the same Go contracts and VSIX
allowlist documented there.

## Security Vulnerabilities

Please review [our security policy](SECURITY.md). Never open a public issue for
a vulnerability.

## License

Open-sourced software licensed under the [MIT license](LICENSE.md).
