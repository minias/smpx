# 보안 정책 (Security Policy)

[English](SECURITY.md) | **한국어**

## 지원 버전

보안 수정은 `main` 에 반영되고 다음 릴리스로 배포됩니다. 최신 릴리스만 지원합니다.

| 버전 | 지원 |
| ---- | ---- |
| 최신 릴리스 / `main` | :white_check_mark: |
| 이전 릴리스 | :x: |

## 취약점 신고

보안 취약점은 공개 이슈로 올리지 말고 GitHub 의
[비공개 취약점 신고(Private vulnerability reporting)](https://github.com/minias/smpx/security/advisories/new)
로 제보해 주세요.

- 접수 후 7일 이내에 확인 결과를 회신합니다.
- 취약점으로 확인되면 수정 버전을 배포한 뒤 Security Advisory 로 공개합니다.
- 취약점이 아니라고 판단되면 그 이유를 회신합니다.

SMPX 는 외부 프로그램 `yt-dlp` 를 내려받아 실행하므로, yt-dlp 자체의 취약점은 [yt-dlp 프로젝트](https://github.com/yt-dlp/yt-dlp/security)에 함께 제보해 주세요.

## 저장소 보안 조치

| 영역 | 조치 |
|---|---|
| 의존성 | Dependabot 취약점 알림 · 보안 업데이트 사용, 버전 업데이트 PR 은 `dev` 로, main push 마다 Go 의존성 그래프 제출 |
| 비밀 정보 | GitHub 시크릿 스캔 + push 보호, 모든 PR / push 에서 gitleaks 검사 |
| 코드 분석 | CodeQL 기본 스캔 (Go, JavaScript/TypeScript, Actions), 매주 + PR 마다 |
| `main` 브랜치 | 보호 규칙: 강제 push 금지, CodeQL 결과 필요 |
| 릴리스 태그 | 보호 규칙: `v*` 태그 삭제 · 이동 금지 |
| CI 권한 | 워크플로 기본 권한 `contents: read`, 릴리스 작업만 쓰기 권한 |
| 서명 | 서명 인증서는 GitHub Secrets 에만 보관, 저장소에 포함하지 않음 |

## 앱 보안 설계

- **yt-dlp**: 공식 GitHub 릴리스에서 내려받아 `SHA2-256SUMS` 로 검증한 뒤 설치, yt-dlp 자체 업데이터로 갱신
- **스트림 프록시**: `127.0.0.1` 에서만 대기하며, 앱이 등록한 주소만 무작위 128비트 토큰으로 중계 (임의 주소 프록시 아님)
- **외부 프로세스 실행**: 셸 없이 인자 배열로 실행, `-` 로 시작하는 URL 거부 및 URL 앞에 `--` 전달
- **로컬 데이터**: 사용자 설정 폴더의 `playlist.json`(제목, YouTube URL)만 저장 — 계정 · 쿠키 · 사용 통계 수집 없음

## 알려진 수용 위험

| 권고 | 수용 사유 |
|---|---|
| `deepmerge` 프로토타입 오염 (low, 아직 수정 버전 없음) | `@sveltejs/vite-plugin-svelte` 가 빌드할 때 자체 설정을 합치는 데만 사용 — 앱에 포함되지 않고 외부 입력을 받지 않음. 수정 버전이 나오면 바로 업데이트합니다. |
