# Agent Instructions

## Branch and pull request policy

- **Never push directly to `master` or `dev`.** Every change, including docs,
  version bumps, and release fixes, goes through a pull request.
- Create a topic branch from an up-to-date `dev`, push the branch, and open a PR
  against `dev` with `gh pr create --base dev`.
- Wait for CI to pass, then merge the topic PR into `dev`.
- Promote `dev` to `master` through a separate `dev` → `master` pull request.
  Do not fast-forward `master` locally and push.
- **Never merge a pull request that targets `master`.** Promotion PRs are merged
  manually by the repository owner. Open the PR, verify CI, and leave it open
  for their review. Do not run `gh pr merge` on a `master` PR.
- Tags and releases are cut from `master` only after the owner merges the
  promotion PR and CI is green.

## Local verification

Run the checks that CI runs before opening a PR:

```bash
gofmt -l internal main.go
go vet -tags gtk3 ./internal/... .
go test -tags gtk3 -race ./internal/... .
npm --prefix frontend run build
```

Local machines may only have GTK3 headers; release CI builds against GTK4.

## Version bumps

Use the single version script so Go, npm, Linux packaging, and macOS plists stay
in sync:

```bash
node scripts/set-version.mjs <semver>
```

## Release assets

Every tag publishes Linux packages (AppImage, DEB, RPM, Arch), a universal
macOS DMG, and a Windows installer plus portable EXE. The updater matches the
AppImage by platform and architecture, so keep the
`asdf-mise-gui-linux-<arch>.AppImage` naming and the `checksums.txt` sidecar in
`.github/workflows/release.yml`, or AppImage self-update stops working.
