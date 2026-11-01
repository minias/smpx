# Security Policy

**English** | [한국어](SECURITY_ko.md)

## Supported Versions

Security fixes are made on `main` and shipped in the next release. Only the latest release is supported.

| Version | Supported |
| ------- | --------- |
| Latest release / `main` | :white_check_mark: |
| Older releases | :x: |

## Reporting a Vulnerability

Please do not open a public issue for security vulnerabilities. Report them through GitHub
[Private vulnerability reporting](https://github.com/minias/smpx/security/advisories/new).

- You will receive a response within 7 days.
- If the vulnerability is confirmed, a fixed version is released and then disclosed as a Security Advisory.
- If it is not considered a vulnerability, you will get an explanation.

SMPX downloads and runs the external program `yt-dlp`. Please report vulnerabilities in yt-dlp itself to the [yt-dlp project](https://github.com/yt-dlp/yt-dlp/security) as well.

## Security measures in this repository

| Area | Measure |
|---|---|
| Dependencies | Dependabot alerts and security updates enabled; version-update PRs go to `dev`; Go dependency graph submitted on every push to `main` |
| Secrets | GitHub secret scanning with push protection, plus a gitleaks scan on every PR / push |
| Code analysis | CodeQL default setup (Go, JavaScript/TypeScript, Actions), weekly and on PRs |
| `main` branch | Ruleset: no force push, CodeQL results required |
| Release tags | Ruleset: `v*` tags cannot be deleted or moved |
| CI permissions | Workflows run with `contents: read` by default; only the release job can write |
| Signing | Signing certificates are stored only in GitHub Secrets, never in the repository |

## Security design of the app

- **yt-dlp**: downloaded from the official GitHub release and verified against `SHA2-256SUMS` before it is installed; updated with yt-dlp's own updater.
- **Stream proxy**: listens on `127.0.0.1` only and relays only URLs the app registered, each behind a random 128-bit token (not an open proxy).
- **External processes**: launched with an argument list (no shell); URLs starting with `-` are rejected and `--` is passed before the URL.
- **Local data**: only `playlist.json` (titles and YouTube URLs) in the user config directory; no accounts, cookies or telemetry.

## Known accepted risks

| Advisory | Why it is accepted |
|---|---|
| `deepmerge` prototype pollution (low, no fixed version yet) | Used only at build time by `@sveltejs/vite-plugin-svelte` to merge its own configuration; not shipped in the app and never receives external input. Will be updated as soon as a fix is released. |
