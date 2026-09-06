# Publishing

The extension is published to **Open VSX**, the registry the editors built on
VS Code read: Cursor, Windsurf, Antigravity, VSCodium and Theia. Microsoft's own
marketplace refuses those editors by its terms of service, so Open VSX is the
only registry that reaches them.

## Whose account publishes

Publishing is done from **`paulorlima@hyz.is`**, in every registry. Both
addresses belong to HYZIS - SERVICOS DIGITAIS LTDA - EPP, which is the company
that holds the copyright: `paulorlima@` is the named one and publishes,
`admin@` is the internal one and does not.

So the account that uploads, the identity on the commits, and the entity on the
licence are all the same company, reached by the same address. That is the point
of writing it down: an account chosen ad hoc for one release is the one nobody
can find when the next release needs it.

This matters beyond tidiness: claiming ownership of the namespace is a public
issue on the registry's repository, and a namespace held by a company while a
personal account publishes into it is the first thing a reader asks about.

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
4. Create the namespace. The name is the `publisher` field of the manifest,
   exactly — `arandu-io`, not the display name. A namespace name may hold
   letters, digits, and `_ - + $ ~`.

   ```
   npx ovsx create-namespace arandu-io --pat <token>
   ```

5. **Claim ownership of it, which creating it does not give you.** Creating a
   namespace makes you a *contributor*: you can publish, and every version you
   publish is shown as **unverified**, with a warning icon on the extension page.
   Ownership is what turns that into the verified shield, and it is granted
   publicly — by opening an issue at
   <https://github.com/EclipseFdn/open-vsx.org>. Granting it in the open is the
   point: anyone can contest a claim by commenting on the issue.

   Until this is granted, the extension is installable and marked unverified.

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

Microsoft's marketplace is what VS Code itself reads, and no other editor can.
Open VSX covers the rest. Neither covers both, so a release goes to each —
`make publish-all` does the two from one build.

Once, before the first publish there:

1. Create an organisation at <https://dev.azure.com> with the same account.
2. Under *User settings → Personal Access Tokens*, create a token scoped to
   **Marketplace → Manage**, with **All accessible organizations** selected.
   A token scoped to a single organisation is accepted when created and refused
   when publishing, and the refusal reads as an authentication failure rather
   than a scope one.
3. Create the publisher `arandu-io` at
   <https://marketplace.visualstudio.com/manage>. The name must match the
   `publisher` field, exactly as on Open VSX.

Every release:

```
VSCE_PAT=<token> make publish-marketplace
```
