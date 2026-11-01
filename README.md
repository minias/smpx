# SMPX

[SMP](https://github.com/minias/smp)(Windows 전용 C# WinForms 유튜브 백그라운드 음악 재생기)를
**macOS / Windows / Linux** 에서 동작하도록 Go + [Wails v2](https://wails.io) 로 확장한 프로젝트입니다.

YouTube 링크를 플레이리스트에 추가하면 `yt-dlp` 로 오디오 스트림 주소를 추출하고 `mpv` 로 재생합니다.

> 상태: ✅ SMP 핵심 기능 이식 완료, macOS 검증 / Windows·Linux 는 크로스 컴파일까지 확인 (2026-11-01, `feature/smp-port`)

---

## 기술 스택

| 구분 | SMP (원본) | SMPX |
|---|---|---|
| 언어 / UI | C# .NET, WinForms | Go 1.25, Wails v2.12, Svelte 3 |
| 오디오 | NAudio (MediaFoundation) | mpv (외부 프로세스 + JSON IPC) |
| 스트림 추출 | yt-dlp.exe | yt-dlp |
| 플랫폼 | Windows | macOS / Windows / Linux |

---

## 아키텍처

SMP 의 클린 아키텍처 구조(UI → UseCase → Domain / Infrastructure)를 그대로 따릅니다.

```
app.go / main.go                 Wails 바인딩 (UI → Service 호출만 담당)
frontend/                        Svelte UI
internal/
  domain/                        LoopMode, PlaybackState, PlaylistItem
  application/
    player/                      AudioPlayer 포트
    youtube/                     Resolver 포트 (제목 / 스트림 URL)
    playlist/                    Repository 포트
    service/                     PlayerService (SMP PlayerFacade + UseCases), PlayerState
  infrastructure/
    runtime/                     OS별 실행파일 탐색, IPC 경로, 콘솔창 숨김
    player/                      mpv 프로세스 + IPC (Unix socket / Windows named pipe)
    ytdlp/                       yt-dlp 클라이언트 (TTL 캐시, 중복 요청 방지)
    storage/                     playlist.json 저장소
    update/                      GitHub Release 업데이트 확인
resources/bin/<os>/              번들용 mpv, yt-dlp 실행파일
```

---

## SMP 기능 이식 현황

| SMP 기능 | SMPX 구현 | 상태 |
|---|---|---|
| 플레이리스트 추가 (제목 자동 조회, 중복 방지) / 삭제 | `PlayerService.AddTrack / RemoveTrack` | ✅ |
| 플레이리스트 저장 / 로드 (`playlist.json`, 같은 포맷) | `storage` — `UserConfigDir/smpx/playlist.json`, 원자적 쓰기 | ✅ |
| 재생 / 일시정지 / 재개 / 정지 | `PlayerService` + mpv IPC | ✅ |
| 이전곡 / 다음곡 | `Previous / Next` | ✅ |
| 루프 모드 (1회 / 전체 / 1곡) | `CycleLoopMode`, 곡 종료 시 자동 다음곡 | ✅ |
| 볼륨 조절 | `SetVolume` (0~100, 다음 곡에도 유지) | ✅ |
| 스트림 URL 캐시 (TTL 30분) + 중복 요청 방지 | `ytdlp` | ✅ |
| 재생곡 알림 / 창 제목 표시 | Wails 알림 API (macOS 는 번들 앱에서만) | ✅ |
| 미디어 키 (재생/일시정지, 이전, 다음) | mpv `input-media-keys` → IPC `client-message` (mpv 빌드/OS 에 따라 동작) | ✅ |
| 키보드 단축키 | 앱 포커스 시 Space / ← → / ↑ ↓ | ✅ |
| 업데이트 확인 | GitHub Release 확인 + 다운로드 링크 (자동 설치는 미지원) | ✅ |
| 시스템 트레이 | Wails v2 미지원 → 보류 (Wails v3 이전 시 검토) | ⏸ |
| 볼륨키 연타로 이전/다음곡 (MediaKeyPatternDetector) | 전역 키 후킹이 OS별로 필요 → 보류 | ⏸ |

---

## 작업 방식

메인 에이전트가 Domain / Application 레이어(포트 인터페이스, `PlayerService`)를 먼저 작성해 계약을 고정하고,
나머지를 서브 에이전트 5개가 병렬로 구현합니다. 메인 에이전트는 결과 취합과 검증을 담당합니다.

| # | 담당 | 범위 |
|---|---|---|
| 1 | runtime | 실행파일 탐색(.app 번들 / 설치 폴더 / 개발 경로 / PATH), IPC 경로, Windows 콘솔창 숨김 |
| 2 | player | mpv 오디오 전용 재생, IPC, 곡 종료 감지, 미디어 키, Windows named pipe |
| 3 | ytdlp | 제목 / 스트림 URL 추출, TTL 캐시, 중복 요청 방지, 단위 테스트 |
| 4 | storage + update | playlist.json 저장소, GitHub Release 확인, 단위 테스트 |
| 5 | presentation | Wails 바인딩, 의존성 연결, Svelte UI, 바인딩 재생성 |

---

## 검증 결과 (macOS, 2026-11-01)

| 항목 | 결과 |
|---|---|
| `go build ./...`, `go vet ./...` (darwin / windows / linux) | ✅ |
| `go test -race ./...` — service · runtime · ytdlp · storage · update | ✅ |
| 실제 yt-dlp + mpv 연동: 제목 조회 → 재생(약 2초) → 일시정지/재개 → 곡 종료 후 자동 정지 | ✅ |
| `wails build` 후 앱 실행 / 창 표시 / 정상 종료 | ✅ |
| Windows named pipe IPC, Windows/Linux 실행 | ⚠️ 컴파일만 확인, 실기기 테스트 필요 |
| 미디어 키 | ⚠️ IPC 경로는 확인, 실제 OS 미디어 키 전달은 mpv 빌드에 따라 다름 |

## 남은 과제

- `wails build` 결과물(.app / 설치본)에 mpv · yt-dlp 번들링 (현재는 Homebrew / PATH 로 대체 탐색)
- 시스템 트레이, 볼륨키 패턴 감지 (위 표의 보류 항목)
- 저장소 정리: 작업 폴더 전체가 CRLF 로 바뀌어 있음, `frontend/node_modules`·`build/bin` 이 커밋되어 있음

## 개발

필요: Go 1.25+, Node.js, [Wails CLI](https://wails.io/docs/gettingstarted/installation), mpv, yt-dlp

```bash
wails dev      # 개발 모드 (프로젝트 루트의 resources/bin 또는 PATH 의 mpv / yt-dlp 사용)
wails build    # 배포용 빌드
go test ./...
```

> ExFAT 등 심볼릭 링크를 지원하지 않는 드라이브에서도 빌드되도록 `frontend/package.json` 스크립트는 `node node_modules/vite/bin/vite.js` 를 직접 실행합니다.

실행파일 탐색 순서: 실행파일 폴더 → (macOS) `.app/Contents/Resources/bin` → `resources/bin/<os>` → `PATH` → (macOS) Homebrew 경로

## 라이선스

MIT License
