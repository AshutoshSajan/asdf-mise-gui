# Release signing

The release workflow signs macOS and Windows artifacts automatically when the
repository has the credentials below. Without them it still builds and
publishes the same assets, using the ad-hoc macOS signature and unsigned Windows
binaries that earlier releases used. Linux packages are not code-signed.

## Required secrets

| Secret | Platform | Value |
| :-- | :-- | :-- |
| `APPLE_CERTIFICATE_P12` | macOS | Base64-encoded Developer ID Application certificate and private key (`.p12`) |
| `APPLE_CERTIFICATE_PASSWORD` | macOS | Password for the `.p12` |
| `APPLE_ID` | macOS | Apple ID that owns the Developer ID certificate |
| `APPLE_APP_PASSWORD` | macOS | App-specific password for notarization |
| `APPLE_TEAM_ID` | macOS | Apple Developer team identifier |
| `WINDOWS_CERTIFICATE_PFX` | Windows | Base64-encoded code-signing certificate and private key (`.pfx`) |
| `WINDOWS_CERTIFICATE_PASSWORD` | Windows | Password for the `.pfx` |

Signing only runs when every secret for that platform is present. The check is a
job-level environment variable, so pull requests from forks and repositories
without credentials keep the unsigned path.

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
