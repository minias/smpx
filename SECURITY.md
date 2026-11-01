# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| < 0.1   | :x:                |

## Reporting a Vulnerability

보안 취약점은 공개 이슈로 올리지 말고 GitHub 의
[비공개 취약점 신고(Private vulnerability reporting)](https://github.com/minias/smpx/security/advisories/new)
로 제보해 주세요.

- 접수 후 7일 이내에 확인 결과를 회신합니다.
- 취약점으로 확인되면 수정 버전을 배포한 뒤 Security Advisory 로 공개합니다.
- 취약점이 아니라고 판단되면 그 이유를 회신합니다.

SMPX 는 외부 프로그램 `yt-dlp` 를 내려받아 실행하므로(SHA-256 검증), yt-dlp 자체의 취약점은 [yt-dlp 프로젝트](https://github.com/yt-dlp/yt-dlp/security)에 함께 제보해 주세요.
