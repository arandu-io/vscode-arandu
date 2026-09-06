# Publishing

The extension is published to **Open VSX**, the registry the editors built on
VS Code read: Cursor, Windsurf, Antigravity, VSCodium and Theia. Microsoft's own
marketplace refuses those editors by its terms of service, so Open VSX is the
only registry that reaches them.

## Once, before the first publish

Open VSX is run by the Eclipse Foundation, and it will not accept an upload
until the account behind it has signed their agreement. The refusal names the
agreement rather than the token, which is easy to misread as an auth problem.

1. Sign in at <https://open-vsx.org> with the GitHub account that owns
   `arandu-io`.
2. Open the profile and sign the **Eclipse Foundation Publisher Agreement**.
   This is the step that cannot be skipped and cannot be done from a script.
3. Create an access token under *Settings → Access Tokens*. Copy it once — the
   page does not show it again.
4. Claim the namespace, which reserves `arandu-io` so nobody else takes it:

   ```
   npx ovsx create-namespace arandu-io --pat <token>
   ```

## Every release

```
OVSX_PAT=<token> make publish-openvsx
```

The target builds the package first, so what reaches the registry is what the
audit passed, never a file left in `dist/` from an earlier build. `make
publish-check` does the same build and asks the registry what it already has,
without uploading anything.

## What a version means here

A version is permanent: Open VSX does not accept the same one twice, and it
does not delete. Bump `version` in `package.json` before publishing, and cut the
matching git tag.

## The other registry

Publishing to Microsoft's marketplace needs an Azure DevOps organisation, a
verified publisher, and a Personal Access Token scoped to *Marketplace →
Manage*. It reaches VS Code itself and nothing else; Open VSX reaches everything
else. Neither covers both.
