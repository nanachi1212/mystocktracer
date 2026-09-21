# Desktop automatic updates

The packaged macOS and Windows apps use `electron-updater` with this repository's own GitHub
Releases as the update source:
`https://github.com/nanachi1212/mystocktracer/releases`.

Phase B1 replaced the inherited easy-stock Alibaba OSS feed
(`easy-stock-fs.oss-cn-beijing.aliyuncs.com`), which was controlled by the upstream project. The
owner/repo pair lives in `desktop/update-feed.cjs` as a hard constant: there is no environment
variable, setting, or request parameter that can point the updater at another repository or host.

- macOS publishes a signed/notarized ZIP, its blockmap and `latest-mac.yml`; DMGs are published in
  the same release for manual installation.
- Windows publishes a signed NSIS installer, its blockmap and `latest.yml`.
- Every downloaded artifact is verified against the SHA-512 digest recorded in `latest*.yml`;
  `desktop/scripts/verify-updater-artifacts.mjs` re-checks those digests in CI before publishing.
- The app checks 30 seconds after startup and every 12 hours. Downloads and restarts always require
  a user action.
- For updates started inside the app, it stops its local services, flushes Electron sessions, and
  backs up user data outside the Electron `userData` directory before installation. The latest
  three backups are retained.
- A persistence migration/open failure prevents the desktop backend from starting instead of
  silently opening an empty database.

Release signing secrets expected by `.github/workflows/release.yml`:

- macOS: `MAC_CSC_LINK`, `MAC_CSC_KEY_PASSWORD`, `APPLE_ID`, `APPLE_APP_SPECIFIC_PASSWORD`, `APPLE_TEAM_ID`
- Windows: `WIN_CSC_LINK`, `WIN_CSC_KEY_PASSWORD`

No object-storage credentials are required any more; publishing uses the workflow's own
`GITHUB_TOKEN`.

## Legacy installation transition

Versions released before Phase B1 only poll the upstream Alibaba OSS feed. A new client cannot
change that old client's configured feed, and this repository does not control the upstream bucket.
Those installations therefore require a **one-time manual upgrade** from this repository's GitHub
Releases. After that installation, automatic checks use GitHub Releases.

The manual upgrade reuses the existing Electron user-data location and does not intentionally delete
settings, Watchlist, Portfolio, research history, alerts, browser-auth data, or legacy databases.
However, a directly launched NSIS or DMG installer does not run the in-app update manager and does
not create its external backup. Before this one-time manual upgrade, close the app and copy its
current user-data directory to a safe external location. Keep that copy until the upgraded app has
opened successfully and the Taiwan data is visible.

Without signing credentials, CI can still produce packages for smoke testing, but those packages are
not production-ready for unattended replacement. macOS automatic installation requires a Developer
ID signature and notarization; Windows should use Authenticode to avoid an untrusted
installer/update path.

## Release procedure

Every release must be made by pushing a tag matching `desktop/package.json`, or by manually running
the `Desktop Release` workflow with an existing tag. The workflow performs the following steps in
order:

1. Build and verify macOS arm64, macOS x64 and Windows x64 assets.
2. Merge and verify updater metadata.
3. Assemble one GitHub release asset set containing the user downloads (DMG/EXE) **and** the updater
   assets (`latest-mac.yml`, `latest.yml`, ZIPs, blockmaps).
4. Re-verify the updater metadata against the assembled set.
5. Publish the assets plus `SHA256SUMS.txt` to the GitHub Release.

Do not delete updater assets from a published release. The two `latest*.yml` assets are the update
channels, and every file they reference must remain available. To validate a local asset directory:

```bash
node desktop/scripts/verify-updater-artifacts.mjs desktop/dist/release latest-mac.yml latest.yml
```

```bash
node desktop/scripts/prepare-publish-assets.mjs desktop/dist/release /tmp/mystocktracer-publish v0.4.0
```
