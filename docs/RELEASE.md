# Release / Installers

**English** | [한국어](RELEASE_ko.md)

## Artifacts

Pushing a `v*` tag makes the [Release workflow](../.github/workflows/release.yml) build these files and attach them to a GitHub Release.

| File | Target | Install |
|---|---|---|
| `SMPX_<version>_macos_universal.dmg` | macOS 11+ (Intel / Apple Silicon) | Open it and drag `SMPX` to `Applications` |
| `smpx_<version>_linux_amd64.deb` | Ubuntu 22.04+ / Debian 12+ | `sudo apt install ./smpx_<version>_linux_amd64.deb` |
| `smpx_<version>_linux_amd64.tar.gz` | Other distributions | Extract and run `smpx` (requires GTK3, WebKitGTK 4.1, GStreamer) |
| `SHA256SUMS` | Integrity check | `shasum -a 256 -c SHA256SUMS` |

No setup is needed after installing. The app downloads `yt-dlp` on first launch (SHA-256 verified) and updates it daily.
The `.deb` declares GTK / WebKitGTK / GStreamer codecs as dependencies, so apt installs them too.

## Release process

1. Merge work branches → `dev` via PR, then `dev` → `main`
2. On `main`, run `./release.sh` → enter the version → the tag is pushed
3. Watch the build in [Actions](https://github.com/minias/smpx/actions/workflows/release.yml) → the Release is created automatically

PRs that change packaging files (`scripts/`, `build/`, `wails.json`, the workflow) only build (no Release), and the outputs are available as artifacts.

Release tags (`v*`) are protected by a ruleset: they cannot be deleted or moved.

## First launch on macOS

macOS blocks apps that are not notarized the first time they are opened on another Mac. Allow it once and it opens normally afterwards.

1. Move `SMPX` to `Applications` and open it → when "cannot be opened" appears, click **Done**
2. **System Settings → Privacy & Security** → next to "SMPX was blocked", click **Open Anyway**
3. Open it again → **Open**

Or run once in Terminal:

```bash
xattr -dr com.apple.quarantine /Applications/smpx.app
```

## macOS signing / notarization

| Certificate | Cost | Result |
|---|---|---|
| None | Free | Ad-hoc signature, must be allowed once on other Macs |
| **Apple Development** (in use) | Free (Apple ID) | Developer signature, no notarization → must be allowed once on other Macs |
| Developer ID Application | Apple Developer Program, $99/year | Signed + notarized → opens without warnings |

The workflow prefers a Developer ID Application certificate and falls back to Apple Development (no notarization).

> ⚠️ This is a public repository. **Never commit** certificates, passwords, Apple ID or Team ID — register them as Secrets only.

| Secret | Value |
|---|---|
| `MACOS_CERT_P12_BASE64` | Signing certificate (.p12 — Apple Development or Developer ID Application), base64-encoded |
| `MACOS_CERT_PASSWORD` | Password set when exporting the .p12 |
| `APPLE_ID` | Apple Developer account email |
| `APPLE_TEAM_ID` | Team ID (developer.apple.com → Membership) |
| `APPLE_APP_PASSWORD` | App-specific password created at appleid.apple.com |

### 1. Create a certificate

1. Xcode → **Settings** → **Accounts** → select the account → select the team → **Manage Certificates…**
2. **+** at the bottom left → **Apple Development** (free) or **Developer ID Application** (paid Program)
3. Check: the certificate name appears in

```bash
security find-identity -v -p codesigning
```

### 2. Export the certificate as .p12

1. Open **Keychain Access** (Spotlight: "Keychain")
2. Left: **Default Keychains → login** → top tab **My Certificates**
3. Right-click `Apple Development: …` (or `Developer ID Application: …`) → **Export "…"**
   (expand ▶ to make sure the private key is included — it cannot be exported without it)
4. File format **Personal Information Exchange (.p12)** → save as `DeveloperID.p12` on the Desktop
5. Set an export **password** → this is `MACOS_CERT_PASSWORD`
6. Enter your Mac login password if asked (allow keychain access)

### 3. Register GitHub Secrets

> Do not paste passwords or certificate values into chats or files — type them in **your own Terminal**.
> `gh secret set` prompts for the value and does not echo it.

`APPLE_ID` / `APPLE_TEAM_ID` / `APPLE_APP_PASSWORD` are only needed for **notarization with Developer ID**. With an Apple Development certificate, register just the certificate and its password (skip the password if you exported with an empty one).

| Secret | Where to find it |
|---|---|
| `APPLE_ID` | Apple Developer account email |
| `APPLE_TEAM_ID` | developer.apple.com → Account → **Membership details** → Team ID (10 characters) |
| `APPLE_APP_PASSWORD` | appleid.apple.com → **Sign-In and Security** → **App-Specific Passwords** → generate (e.g. `smpx-notary`) |

```bash
# certificate (.p12 → base64)
base64 -i ~/Desktop/DeveloperID.p12 | gh secret set MACOS_CERT_P12_BASE64 --repo minias/smpx

# each of these prompts for the value
gh secret set MACOS_CERT_PASSWORD --repo minias/smpx
gh secret set APPLE_ID --repo minias/smpx
gh secret set APPLE_TEAM_ID --repo minias/smpx
gh secret set APPLE_APP_PASSWORD --repo minias/smpx

# check (names only, values are hidden)
gh secret list --repo minias/smpx

# delete the exported file
rm ~/Desktop/DeveloperID.p12
```

From the next tag release on, the dmg is signed. (With Developer ID + Apple account secrets it is also notarized and opens without warnings.)

## Local packaging

```bash
scripts/package-macos.sh 0.2.0   # macOS: dist/SMPX_0.2.0_macos_universal.dmg (unsigned without signing variables)
scripts/package-linux.sh 0.2.0   # Linux (Ubuntu 22.04+): dist/*.deb, dist/*.tar.gz
```
