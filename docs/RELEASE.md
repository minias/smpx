# 릴리스 / 설치 프로그램

## 결과물

`v*` 태그를 push 하면 [Release 워크플로](../.github/workflows/release.yml)가 아래 파일을 만들어 GitHub Release 에 올립니다.

| 파일 | 대상 | 설치 |
|---|---|---|
| `SMPX_<버전>_macos_universal.dmg` | macOS 11+ (Intel / Apple Silicon) | 열어서 `SMPX` 를 `Applications` 로 드래그 |
| `smpx_<버전>_linux_amd64.deb` | Ubuntu 22.04+ / Debian 12+ | `sudo apt install ./smpx_<버전>_linux_amd64.deb` |
| `smpx_<버전>_linux_amd64.tar.gz` | 기타 배포판 | 압축 해제 후 `smpx` 실행 (GTK3, WebKitGTK 4.1, GStreamer 필요) |
| `SHA256SUMS` | 무결성 확인 | `shasum -a 256 -c SHA256SUMS` |

설치 후 별도 설정은 필요 없습니다. `yt-dlp` 는 최초 실행 시 앱이 내려받고(SHA-256 검증) 하루 한 번 자동 업데이트합니다.
`.deb` 는 실행에 필요한 GTK / WebKitGTK / GStreamer 코덱을 의존성으로 선언해 apt 가 함께 설치합니다.

## 릴리스 절차

1. 작업 브랜치 → `dev` PR 병합 → `dev` → `main` 병합
2. `main` 에서 `./release.sh` 실행 → 버전 입력 → 태그 push
3. [Actions](https://github.com/minias/smpx/actions/workflows/release.yml) 에서 빌드 확인 → Release 자동 생성

패키징 관련 파일(`scripts/`, `build/`, `wails.json`, 워크플로)을 바꾼 PR 은 Release 없이 빌드만 수행해 artifact 로 결과물을 확인할 수 있습니다.

## macOS 서명 / 공증 설정

서명하지 않은 dmg 는 최초 실행 시 macOS 가 "확인되지 않은 개발자" 경고를 띄웁니다.
아래 GitHub Secrets 를 등록하면 워크플로가 Developer ID 로 서명하고 공증(notarization)까지 수행해 경고 없이 열립니다.

> ⚠️ 이 저장소는 공개 저장소입니다. 인증서, 암호, Apple ID, Team ID 는 **절대 파일로 커밋하지 말고** Secrets 에만 등록하세요.

| Secret | 값 |
|---|---|
| `MACOS_CERT_P12_BASE64` | Developer ID Application 인증서(.p12)를 base64 로 인코딩한 값 |
| `MACOS_CERT_PASSWORD` | .p12 내보낼 때 지정한 암호 |
| `APPLE_ID` | Apple Developer 계정 이메일 |
| `APPLE_TEAM_ID` | Team ID (developer.apple.com → Membership) |
| `APPLE_APP_PASSWORD` | appleid.apple.com 에서 만든 앱 암호 |

### 인증서 준비

1. Xcode → Settings → Accounts → Manage Certificates → `+` → **Developer ID Application** 생성
   (또는 developer.apple.com → Certificates 에서 CSR 로 발급)
2. 키체인 접근 → 내 인증서 → `Developer ID Application: …` 우클릭 → 내보내기 → `.p12` (암호 지정)
3. base64 로 변환해 Secret 등록 (파일은 등록 후 삭제)

```bash
base64 -i DeveloperID.p12 | gh secret set MACOS_CERT_P12_BASE64 --repo minias/smpx
gh secret set MACOS_CERT_PASSWORD --repo minias/smpx    # 입력 프롬프트로 값 입력
gh secret set APPLE_ID --repo minias/smpx
gh secret set APPLE_TEAM_ID --repo minias/smpx
gh secret set APPLE_APP_PASSWORD --repo minias/smpx
rm DeveloperID.p12
```

## 로컬 패키징

```bash
scripts/package-macos.sh 0.2.0   # macOS: dist/SMPX_0.2.0_macos_universal.dmg (서명 변수 없으면 서명 없음)
scripts/package-linux.sh 0.2.0   # Linux(Ubuntu 22.04+): dist/*.deb, dist/*.tar.gz
```
