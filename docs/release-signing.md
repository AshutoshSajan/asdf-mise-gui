# Release signing

The release workflow signs macOS and Windows artifacts automatically when the
repository has the credentials below. Without them it still builds and
publishes the same assets, using the ad-hoc macOS signature and unsigned Windows
binaries that earlier releases used. Linux packages are not code-signed.

The publish job also uploads an Ed25519 `.sig` sidecar for each artifact. Builds
that embed the public key require those signatures, so the workflow fails
rather than publishing a release they would reject.

## Required secrets

| Secret | Platform | Value |
| :-- | :-- | :-- |
| `UPDATE_SIGNING_KEY` | Updates (all) | Ed25519 private key in PKCS#8 PEM form; its public half is committed at `internal/version/update-signing-key.pub.pem` |
| `APPLE_CERTIFICATE_P12` | macOS | Base64-encoded Developer ID Application certificate and private key (`.p12`) |
| `APPLE_CERTIFICATE_PASSWORD` | macOS | Password for the `.p12` |
| `APPLE_ID` | macOS | Apple ID that owns the Developer ID certificate |
| `APPLE_APP_PASSWORD` | macOS | App-specific password for notarization |
| `APPLE_TEAM_ID` | macOS | Apple Developer team identifier |
| `WINDOWS_CERTIFICATE_PFX` | Windows | Base64-encoded code-signing certificate and private key (`.pfx`) |
| `WINDOWS_CERTIFICATE_PASSWORD` | Windows | Password for the `.pfx` |

Platform code signing only runs when every secret for that platform is present.
The check is a job-level environment variable, so pull requests from forks and
repositories without credentials keep the unsigned path. Update signing is
separate and mandatory: the publish job fails if `UPDATE_SIGNING_KEY` is absent.

## macOS

1. Export the **Developer ID Application** certificate and private key from
   Keychain Access as a `.p12`.
2. Base64-encode it without line breaks and store it as
   `APPLE_CERTIFICATE_P12`:

   ```bash
   base64 < DeveloperID.p12 | tr -d '\n' > certificate.b64
   gh secret set APPLE_CERTIFICATE_P12 < certificate.b64
   ```

3. Create an app-specific password at <https://appleid.apple.com> and set it as
   `APPLE_APP_PASSWORD`. Set `APPLE_ID` to the same account.
4. Find the team identifier in the Apple Developer portal and set
   `APPLE_TEAM_ID`.

The workflow imports the certificate into a temporary keychain, signs the
universal `.app` with the hardened runtime and
`build/darwin/entitlements.plist`, creates the DMG, signs the DMG, submits it to
`notarytool --wait`, and staples the accepted ticket. It then verifies the
signature and Gatekeeper assessment before uploading.

Production entitlements intentionally contain only
`com.apple.security.network.client`: the app talks to GitHub for update checks and
runs user-installed package managers, and distribution builds are not sandboxed.

## Windows

1. Export the code-signing certificate and private key as a `.pfx`. The
   certificate must allow exportable private keys; hardware-token certificates
   cannot be imported into the hosted runner this way.
2. Base64-encode it without line breaks and store it as
   `WINDOWS_CERTIFICATE_PFX`:

   ```powershell
   [Convert]::ToBase64String([IO.File]::ReadAllBytes("certificate.pfx")) |
     Set-Content -NoNewline certificate.b64
   gh secret set WINDOWS_CERTIFICATE_PFX < certificate.b64
   ```

3. Set `WINDOWS_CERTIFICATE_PASSWORD` to the `.pfx` password.

The workflow imports the certificate into `Cert:\CurrentUser\My`, signs the
portable EXE, builds the NSIS installer around that signed EXE, signs the
installer, and verifies both signatures with `signtool /pa`. Timestamping uses
DigiCert's RFC 3161 service.

## Update signing

The updater does not trust `checksums.txt` on its own: a compromised release
could replace both the AppImage and its checksum. Instead, `sign-release.mjs`
signs the raw SHA-256 digest of every artifact with Ed25519 and writes the
base64 signature to `<artifact>.sig`. The app verifies the `.sig` sidecar
against the public key embedded in `internal/version`, and Wails rejects the
download if the signature is missing or does not match. This is fail-closed:
once a build pins the key, it will not install an unsigned release.

Generate the keypair once and store the private key only in the repository
secret. Keep an offline backup; if the key is lost, existing builds cannot
verify any future release and users must update manually.

```bash
openssl genpkey -algorithm ED25519 -out update-signing-key.pem
openssl pkey -in update-signing-key.pem -pubout \
  -out internal/version/update-signing-key.pub.pem
gh secret set UPDATE_SIGNING_KEY < update-signing-key.pem
rm update-signing-key.pem   # after storing the offline backup
```

Commit only the public key. The publish job runs:

```bash
node scripts/sign-release.mjs dist
node scripts/sign-release.mjs --verify dist
```

The script signs every file in `dist`, including `checksums.txt`, and verifies
each signature against the committed public key before the release is
published. It never prints key material. A missing or mismatched
`UPDATE_SIGNING_KEY` fails the job.

### Checking a new secret before tagging

The publish job is tag-gated, so a mistyped secret is otherwise only caught once
a tag exists, leaving that release without assets. Run **Actions → Verify update
signing key → Run workflow** to compare the secret with the committed public key
first. It prints a short fingerprint of each public key and nothing else, and it
fails on a mismatch, a non-Ed25519 key, a missing secret, or a secret that is not
parsable PEM:

```text
committed public key     3adb5b01b40cbc85
UPDATE_SIGNING_KEY public 3adb5b01b40cbc85
UPDATE_SIGNING_KEY matches the committed public key
```

The same check runs locally, without copying the key anywhere:

```bash
UPDATE_SIGNING_KEY="$(cat update-signing-key.pem)" node scripts/check-signing-key.mjs
```

To verify a downloaded release locally, download every asset and both sidecar
types into one directory, then run:

```bash
node scripts/sign-release.mjs --verify ./release-download
```

## Verifying a local download

macOS:

```bash
codesign --verify --strict --verbose=2 "/Applications/asdf-mise-gui.app"
spctl --assess --type execute -vv "/Applications/asdf-mise-gui.app"
xcrun stapler validate asdf-mise-gui-darwin-universal.dmg
```

Windows:

```powershell
Get-AuthenticodeSignature .\asdf-mise-gui-windows-amd64-installer.exe |
  Format-List Status, StatusMessage, SignerCertificate
```

## Rotation

Replace the affected repository secrets. No workflow change is needed. Revoke
the old certificate with Apple or the Windows certificate authority after the
new one is in place. The workflow never prints secret values; the decoded files
live only in `RUNNER_TEMP` on the ephemeral runner.

Rotating `UPDATE_SIGNING_KEY` needs one extra step because installed builds pin
the old public key:

1. Generate the new keypair and commit the new public key.
2. Keep the old private key in `UPDATE_SIGNING_KEY` and publish one transitional
   release signed with it, even though the binary now embeds the new public key.
   Existing clients accept it because it still verifies with the key they pin;
   the transitional build trusts the new key for later releases. Run both
   signing commands with the old public key for this release (temporarily adjust
   the publish step or run the release manually):

   ```bash
   node scripts/sign-release.mjs --public-key old-update-signing-key.pub.pem dist
   node scripts/sign-release.mjs --verify --public-key old-update-signing-key.pub.pem dist
   ```

3. Switch `UPDATE_SIGNING_KEY` to the new private key, run **Verify update
   signing key** to confirm the secret matches the committed public key, then
   publish subsequent releases normally.

Never replace the embedded key and the secret in the same release without that
transitional step, or already-installed clients will reject every future
update.

Rotating before the first signed release ships needs no transitional release:
no published build pins the old key yet, so the public key and the secret can be
replaced together. `v0.3.0` predates the pin, which is why the first signed
release can use a fresh key.
