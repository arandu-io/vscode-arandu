<p align="center">
  <img src="images/icon.png" alt="Arandu" width="140" height="140">
</p>

<h1 align="center">Arandu for Visual Studio Code</h1>

<p align="center">First-party Kyse language support for Arandu projects.</p>

<p align="center">
<a href="https://github.com/arandu-io/vscode-arandu/actions/workflows/ci.yml"><img src="https://github.com/arandu-io/vscode-arandu/actions/workflows/ci.yml/badge.svg" alt="Build Status"></a>
<a href="https://github.com/arandu-io/vscode-arandu/releases"><img src="https://img.shields.io/github/v/release/arandu-io/vscode-arandu?label=version" alt="Latest Version"></a>
<a href="LICENSE.md"><img src="https://img.shields.io/github/license/arandu-io/vscode-arandu" alt="License"></a>
</p>

## Preview

The extension owns `.kyse.go` files and supplies Kyse syntax highlighting,
comment and indentation rules, and snippets for complete views, layouts,
control flow, composition, CSRF, and both interpolation forms.

This first release is declarative: it has no extension-host runtime and no
local Node dependency graph. Language intelligence remains in the Aru
toolchain and will connect through the editor adapter as that public contract
lands.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). CI runs the same Go contracts and VSIX
allowlist documented there.

## Security Vulnerabilities

Please review [our security policy](SECURITY.md). Never open a public issue for
a vulnerability.

## License

Open-sourced software licensed under the [MIT license](LICENSE.md).
