# SMPX

[SMP](https://github.com/minias/smp)(Windows 전용 C# WinForms 유튜브 백그라운드 음악 재생기)를
**macOS / Linux** 에서 동작하도록 Go + [Wails v2](https://wails.io) 로 확장한 프로젝트입니다.
Windows 는 SMP 를 사용합니다.

YouTube 링크를 플레이리스트에 추가하면 `yt-dlp` 로 오디오 스트림 주소를 추출하고, 앱 화면(WebView)의 HTML5 오디오로 재생합니다.
**사용자가 별도로 설치할 것이 없습니다.** `yt-dlp` 는 최초 실행 시 앱이 내려받고 하루 한 번 자동 업데이트합니다.

> 상태: ✅ SMP 기능 이식 + 무설정 배포 + 화면 보기 + 재생목록 가져오기 (2026-11-01)

---

## 설치

[Releases](https://github.com/minias/smpx/releases) 에서 받아 설치하면 바로 사용할 수 있습니다. (yt-dlp 는 최초 실행 시 자동 설치)

| OS | 파일 | 설치 |
|---|---|---|
| macOS 11+ (Intel / Apple Silicon) | `SMPX_<버전>_macos_universal.dmg` | 열어서 `SMPX` 를 `Applications` 로 드래그 |
| Ubuntu 22.04+ / Debian 12+ | `smpx_<버전>_linux_amd64.deb` | `sudo apt install ./smpx_<버전>_linux_amd64.deb` |
| 기타 Linux | `smpx_<버전>_linux_amd64.tar.gz` | 압축 해제 후 `smpx` 실행 |

릴리스 절차, macOS 서명 / 공증 설정은 [docs/RELEASE.md](docs/RELEASE.md) 참고.

---

## 기술 스택

| 구분 | SMP (원본) | SMPX |
|---|---|---|
| 언어 / UI | C# .NET, WinForms | Go 1.25, Wails v2.12, Svelte 3 |
| 오디오 | NAudio (MediaFoundation) | WebView HTML5 `<audio>` (opus / m4a) |
| 스트림 추출 | yt-dlp.exe (동봉) | yt-dlp (최초 실행 시 자동 설치 + 자동 업데이트) |
| 미디어 키 | 키보드 훅 | MediaSession (macOS '지금 재생 중', 미디어 키, AirPods) |
| 플랫폼 | Windows | macOS / Linux |

---

## 아키텍처

SMP 의 클린 아키텍처 구조(UI → UseCase → Domain / Infrastructure)를 그대로 따릅니다.
재생 상태는 Go(`PlayerService`)가 관리하고, 실제 소리는 프론트엔드의 `<audio>` 가 냅니다.

```
app.go / main.go                 Wails 바인딩 (UI → Service 호출만 담당)
webview_player.go                AudioPlayer 구현: 메인 <audio> 또는 영상 창으로 재생 명령 전달
video_window.go / video_app.go   화면 보기: 같은 실행파일을 영상 창 모드(SMPX_VIDEO_WINDOW)로 실행, stdin / fd 3 JSON 통신
frontend/                        Svelte UI, <audio>, MediaSession
internal/
  domain/                        LoopMode, PlaybackState, PlaylistItem
  application/
    player/                      AudioPlayer 포트
    youtube/                     Resolver 포트 (제목 / 스트림 URL)
    playlist/                    Repository 포트
    service/                     PlayerService (SMP PlayerFacade + UseCases), PlayerState
  infrastructure/
    ytdlp/                       yt-dlp 클라이언트(TTL 캐시, 중복 요청 방지, 스트림 403 확인 후 클라이언트 우회) + Manager(설치 / 업데이트)
    streamproxy/                 127.0.0.1 스트림 프록시: WebView 의 큰 범위 요청을 2MB 조각으로 나눠 YouTube 403 회피
    storage/                     playlist.json 저장소
    update/                      GitHub Release 업데이트 확인
```

```
재생 요청 → PlayerService → yt-dlp(스트림 URL) → "audio:command" 이벤트 → <audio>.play()
<audio> ended / error / pause → App.AudioEnded / AudioFailed / AudioPaused → PlayerService (루프 모드 처리)
```

### yt-dlp 관리 (`internal/infrastructure/ytdlp/manager.go`)

- 위치: `~/Library/Application Support/smpx/bin/yt-dlp` (Linux: `~/.config/smpx/bin/yt-dlp`)
- 최초 실행: GitHub 최신 릴리스의 단일 실행파일(`yt-dlp_macos` / `yt-dlp_linux`) 다운로드 + `SHA2-256SUMS` 검증
- 하루 한 번 `yt-dlp -U` 자동 업데이트, 재생 실패 시 즉시 업데이트 후 1회 재시도 (YouTube 변경 대응)
- 다운로드할 수 없으면 PATH / Homebrew 의 yt-dlp 사용

---

## SMP 기능 이식 현황

| SMP 기능 | SMPX 구현 | 상태 |
|---|---|---|
| 플레이리스트 추가 (제목 자동 조회, 중복 방지) / 삭제 | `PlayerService.AddTrack / RemoveTrack` | ✅ |
| 플레이리스트 저장 / 로드 (`playlist.json`, 같은 포맷) | `storage` — `UserConfigDir/smpx/playlist.json`, 원자적 쓰기 | ✅ |
| 재생 / 일시정지 / 재개 / 정지 | `PlayerService` + WebView `<audio>` | ✅ |
| 이전곡 / 다음곡 | `Previous / Next` | ✅ |
| 루프 모드 (1회 / 전체 / 1곡) | `CycleLoopMode`, 곡 종료 시 자동 다음곡 | ✅ |
| 볼륨 조절 | `SetVolume` (0~100, 다음 곡에도 유지) | ✅ |
| 스트림 URL 캐시 (TTL 30분) + 중복 요청 방지 | `ytdlp` | ✅ |
| 재생곡 알림 / 창 제목 표시 | Wails 알림 API (macOS 는 번들 앱에서만) | ✅ |
| 미디어 키 (재생/일시정지, 이전, 다음) | MediaSession — macOS '지금 재생 중' / 미디어 키 / AirPods | ✅ |
| 키보드 단축키 | 메인 창: Space 재생/일시정지, ← → 이전/다음, ↑ ↓ 볼륨, V 화면 보기 · 영상 창: Enter 전체 화면 전환, Esc 전체 화면 해제, Space 누르는 동안 2배속, 클릭 재생/일시정지 | ✅ |
| 업데이트 확인 | GitHub Release 확인 + 다운로드 링크 (자동 설치는 미지원) | ✅ |
| 재생목록 URL 가져오기 | `list=` URL 이면 목록 전체 추가 (최대 500곡, 비공개/삭제 영상 제외, 영상 ID 기준 중복 제외) | ✅ |
| 화면 보기 (별도 창) | '📺 화면 보기' 체크 / V 키 → 영상 창(자식 프로세스), 영상 비율에 맞춰 창 크기 조정, 켜고 끌 때 이어서 재생 | ✅ |
| 시스템 트레이 | Wails v2 미지원 → 보류 | ⏸ |
| 볼륨키 연타로 이전/다음곡 (MediaKeyPatternDetector) | 전역 키 후킹 필요 → 보류 | ⏸ |

---

## 검증 결과 (macOS, 2026-11-01)

| 항목 | 결과 |
|---|---|
| `go build ./...`, `go vet ./...` (darwin / linux) | ✅ |
| `go test -race ./...` — service · ytdlp(Manager 포함) · storage · update | ✅ |
| 앱 실행 → 재생(Space) → WebView 오디오 출력 → 곡 종료 후 자동 다음곡(사용자 입력 없이) → 마지막 곡 후 정지 | ✅ |
| 오디오 형식: opus 는 길이 정상, m4a 는 WKWebView 에서 길이 2배 → opus 우선 | ✅ |
| 화면 보기: 영상 창 표시 → 영상+소리 재생 → 영상 비율로 창 크기 맞춤 → 체크 해제 / 창 닫기 시 오디오로 이어서 재생 | ✅ |
| 재생목록 URL(40곡) 붙여넣기 → 39곡 추가 + 기존 1곡 중복 제외, 목록 / playlist.json 반영 | ✅ |
| yt-dlp 최초 다운로드 / 자동 업데이트 실제 동작 | ⚠️ 단위 테스트(httptest)만 확인 |
| 스트림 403 대응: 큰 범위 요청 분할(프록시) + 막힌 스트림은 android_vr 클라이언트로 재시도 → 실패하던 영상의 오디오 / 화면 보기 재생 확인 | ✅ |
| 일부 영상은 모든 클라이언트에서 403 | ⚠️ "YouTube가 이 영상의 스트림을 차단했습니다" 표시 |
| Linux 실행 | ⚠️ 컴파일만 확인 |

## 남은 과제

- 영상 창은 별도 프로세스라 Dock 에 아이콘이 하나 더 표시됨 (Wails v3 이전 시 해결 가능)
- macOS 서명 / 공증: GitHub Secrets 등록 시 Release 워크플로가 자동 수행 ([docs/RELEASE.md](docs/RELEASE.md)), 미등록 시 최초 1회 '그래도 열기' 필요
- 시스템 트레이, 볼륨키 패턴 감지

---

## 개발

필요: Go 1.25+, Node.js, [Wails CLI](https://wails.io/docs/gettingstarted/installation) (yt-dlp 는 앱이 자동 설치)

```bash
wails dev      # 개발 모드
wails build    # 배포용 빌드 (build/bin/smpx.app)
go test ./...
```

> ExFAT 등 심볼릭 링크를 지원하지 않는 드라이브에서도 빌드되도록 `frontend/package.json` 스크립트는 `node node_modules/vite/bin/vite.js` 를 직접 실행합니다.
> `frontend/public/.gitkeep` 은 빌드 시 `frontend/dist/.gitkeep` 으로 복사되어 go:embed 디렉토리를 유지합니다.

## 라이선스

MIT License
