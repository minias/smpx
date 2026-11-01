# SMPX

**English** | [한국어](README_ko.md)

SMPX extends [SMP](https://github.com/minias/smp) — a Windows-only C# WinForms YouTube background music player —
to **macOS / Linux**, rebuilt with Go + [Wails v2](https://wails.io). On Windows, use SMP.

Add YouTube links to the playlist and SMPX extracts the audio stream with `yt-dlp` and plays it through the app's WebView (HTML5 audio).
**Nothing else to install.** The app downloads `yt-dlp` on first launch and keeps it updated daily.

> Status: ✅ SMP feature port complete — zero-setup install (macOS dmg / Linux deb), video window, playlist import, menu bar / tray (2026-11-01)

---

## Install

Download from [Releases](https://github.com/minias/smpx/releases) and install — ready to use. (`yt-dlp` is installed automatically on first launch.)

| OS | File | Install |
|---|---|---|
| macOS 11+ (Intel / Apple Silicon) | `SMPX_<version>_macos_universal.dmg` | Open it and drag `SMPX` to `Applications` |
| Ubuntu 22.04+ / Debian 12+ | `smpx_<version>_linux_amd64.deb` | `sudo apt install ./smpx_<version>_linux_amd64.deb` |
| Other Linux | `smpx_<version>_linux_amd64.tar.gz` | Extract and run `smpx` |

> **First launch on macOS**: the app is not notarized, so the first time you need **System Settings → Privacy & Security → Open Anyway**
> (or run `xattr -dr com.apple.quarantine /Applications/smpx.app` in Terminal). After that it opens normally.

See [docs/RELEASE.md](docs/RELEASE.md) for the release process and macOS signing / notarization setup.

---

## Tech stack

| | SMP (original) | SMPX |
|---|---|---|
| Language / UI | C# .NET, WinForms | Go 1.26, Wails v2.17, Svelte 3 |
| Audio | NAudio (MediaFoundation) | WebView HTML5 `<audio>` (opus / m4a) |
| Stream extraction | yt-dlp.exe (bundled) | yt-dlp (auto-installed on first launch + auto-updated) |
| Media keys | Keyboard hook | MediaSession (macOS Now Playing, media keys, AirPods) |
| Platforms | Windows | macOS / Linux |

---

## Architecture

SMPX keeps SMP's clean architecture (UI → UseCase → Domain / Infrastructure).
Go (`PlayerService`) owns the playback state; the frontend's `<audio>` element produces the sound.

```
app.go / main.go                 Wails bindings (UI only calls the service)
webview_player.go                AudioPlayer: sends playback commands to the main <audio> or the video window
video_window.go / video_app.go   Video window: runs the same binary in video mode (SMPX_VIDEO_WINDOW), JSON over stdin / fd 3
tray*.go / tray_darwin.m         Menu bar (macOS NSStatusItem) / tray (Linux StatusNotifierItem)
frontend/                        Svelte UI, <audio>, MediaSession
internal/
  domain/                        LoopMode, PlaybackState, PlaylistItem
  application/
    player/                      AudioPlayer port
    youtube/                     Resolver port (title / stream URL)
    playlist/                    Repository port
    service/                     PlayerService (SMP PlayerFacade + use cases), PlayerState
  infrastructure/
    ytdlp/                       yt-dlp client (TTL cache, request dedup, 403 probe + client fallback) + Manager (install / update)
    streamproxy/                 127.0.0.1 stream proxy: splits WebView's large range requests into 2MB chunks to avoid YouTube 403
    storage/                     playlist.json repository
    update/                      GitHub Release update check
```

```
Play → PlayerService → yt-dlp (stream URL) → "audio:command" event → <audio>.play()
<audio> ended / error / pause → App.AudioEnded / AudioFailed / AudioPaused → PlayerService (loop mode handling)
```

### yt-dlp management (`internal/infrastructure/ytdlp/manager.go`)

- Location: `~/Library/Application Support/smpx/bin/yt-dlp` (Linux: `~/.config/smpx/bin/yt-dlp`)
- First launch: downloads the standalone binary (`yt-dlp_macos` / `yt-dlp_linux`) from the latest GitHub release and verifies it against `SHA2-256SUMS`
- Runs `yt-dlp -U` once a day; on a playback failure it updates immediately and retries once (keeps up with YouTube changes)
- Falls back to `yt-dlp` from PATH / Homebrew if the download is not possible

---

## SMP feature port

| SMP feature | SMPX implementation | Status |
|---|---|---|
| Add (auto title, no duplicates) / remove playlist items | `PlayerService.AddTrack / RemoveTrack` | ✅ |
| Save / load playlist (`playlist.json`, same format) | `storage` — `UserConfigDir/smpx/playlist.json`, atomic writes | ✅ |
| Play / pause / resume / stop | `PlayerService` + WebView `<audio>` | ✅ |
| Previous / next track | `Previous / Next` | ✅ |
| Loop mode (once / all / single) | `CycleLoopMode`, auto-advance when a track ends | ✅ |
| Volume | `SetVolume` (0–100, kept for the next track) | ✅ |
| Stream URL cache (30 min TTL) + request dedup | `ytdlp` | ✅ |
| Now-playing notification / window title | Wails notifications (macOS: bundled app only) | ✅ |
| Media keys (play/pause, previous, next) | MediaSession — macOS Now Playing / media keys / AirPods | ✅ |
| Keyboard shortcuts | Main window: Space play/pause, ← → previous/next, ↑ ↓ volume, V video window · Video window: Enter toggles full screen, Esc exits full screen, hold Space for 2× speed, click to play/pause | ✅ |
| Update check | GitHub Release check + download link (no auto-install) | ✅ |
| Import YouTube playlist URLs | `list=` URLs add the whole list (up to 500, skips private/deleted videos, dedup by video ID) | ✅ |
| Video window | '📺 화면 보기' checkbox / V key → video window (child process, no Dock icon), sized to the video's aspect ratio, resumes position when toggled | ✅ |
| System tray | macOS menu bar (NSStatusItem) / Linux tray (StatusNotifierItem): play, previous, next, open, quit; on macOS closing the window hides it to the menu bar | ✅ |
| Volume-key double press for previous/next (MediaKeyPatternDetector) | Replaced by MediaSession — AirPods double press = next / triple press = previous, media keys (no global key hook, no permission prompt) | ✅ |

---

## Verification (2026-11-01)

| Item | Result |
|---|---|
| `go build ./...`, `go vet ./...` (darwin / linux) | ✅ |
| `go test -race ./...` — service · ytdlp (incl. Manager) · storage · update · streamproxy | ✅ |
| Launch → play (Space) → WebView audio → auto-advance without user input → stop after the last track | ✅ |
| Audio format: opus reports the right duration, m4a reports 2× duration in WKWebView → prefer opus | ✅ |
| Video window: open → video + audio → window fitted to the video → uncheck / close → continues as audio | ✅ |
| Paste a 40-track playlist URL → 39 added + 1 existing skipped, list / playlist.json updated | ✅ |
| yt-dlp first-run install (download + SHA-256 check) | ✅ CI: installed deb app run under xvfb |
| Stream 403 handling: chunked range proxy + android_vr client fallback → previously failing video plays (audio / video window) | ✅ |
| Some videos return 403 for every client | ⚠️ Shows "YouTube가 이 영상의 스트림을 차단했습니다" |
| Linux: build deb → `apt install` → run app | ✅ CI (Ubuntu 22.04) |
| macOS: menu bar (play, next, open), close window → hidden to menu bar → reopen, no Dock icon for the video window | ✅ |

---

## Development

Requires Go 1.26+, Node.js, [Wails CLI](https://wails.io/docs/gettingstarted/installation) (the app installs `yt-dlp` itself)

```bash
wails dev      # development mode
wails build    # release build (build/bin/smpx.app)
go test ./...
```

> `frontend/package.json` scripts run `node node_modules/vite/bin/vite.js` directly so builds also work on drives without symlink support (e.g. ExFAT).
> `frontend/public/.gitkeep` is copied to `frontend/dist/.gitkeep` on build to keep the go:embed directory.

## License

MIT License
