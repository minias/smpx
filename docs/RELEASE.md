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

### 1. Developer ID Application 인증서 만들기

> Xcode 에서 기본으로 만들어지는 **Apple Development** 인증서는 개발용이라 배포용 서명에 쓸 수 없습니다.
> 앱 스토어 밖에서 배포하려면 **Developer ID Application** 인증서가 필요합니다. (Apple Developer Program 계정 소유자만 생성 가능)

1. Xcode → **설정(Settings)** → **Accounts** → 계정 선택 → 팀 선택 → **Manage Certificates…**
2. 왼쪽 아래 **+** → **Developer ID Application** 선택
3. 확인: 터미널에서 아래 명령에 `Developer ID Application: …` 이 보이면 성공

```bash
security find-identity -v -p codesigning
```

### 2. 인증서를 .p12 파일로 내보내기

1. **키체인 접근(Keychain Access)** 앱 실행 (Spotlight 에서 "키체인")
2. 왼쪽 **기본 키체인 → 로그인** 선택 → 위쪽 탭에서 **내 인증서** 선택
3. `Developer ID Application: <이름> (<Team ID>)` 항목을 우클릭 → **"…" 내보내기**
   (항목 왼쪽 ▶ 를 펼쳐 개인 키가 함께 있는지 확인 — 개인 키가 없으면 내보낼 수 없음)
4. 파일 포맷 **개인 정보 교환(.p12)** → 바탕화면에 `DeveloperID.p12` 로 저장
5. 내보내기 **암호**를 새로 지정 → 이 값이 `MACOS_CERT_PASSWORD`
6. Mac 로그인 암호를 물으면 입력 (키체인 접근 허용)

### 3. GitHub Secrets 등록

> 암호 / 인증서 값은 채팅이나 파일에 붙여넣지 말고, **본인 터미널(터미널.app)** 에서 직접 입력하세요.
> `gh secret set` 은 값을 입력받는 프롬프트를 띄우며, 입력 값은 화면에 표시되지 않습니다.

준비할 값:

| Secret | 확인 위치 |
|---|---|
| `APPLE_ID` | Apple Developer 계정 이메일 |
| `APPLE_TEAM_ID` | developer.apple.com → Account → **Membership details** → Team ID (10자리) |
| `APPLE_APP_PASSWORD` | appleid.apple.com → **로그인 및 보안** → **앱 암호** → 생성 (예: `smpx-notary`) |

```bash
# 인증서 (.p12 → base64)
base64 -i ~/Desktop/DeveloperID.p12 | gh secret set MACOS_CERT_P12_BASE64 --repo minias/smpx

# 아래는 실행하면 값 입력 프롬프트가 뜸
gh secret set MACOS_CERT_PASSWORD --repo minias/smpx
gh secret set APPLE_ID --repo minias/smpx
gh secret set APPLE_TEAM_ID --repo minias/smpx
gh secret set APPLE_APP_PASSWORD --repo minias/smpx

# 등록 확인 (값은 보이지 않고 이름만 표시)
gh secret list --repo minias/smpx

# 내보낸 파일 삭제
rm ~/Desktop/DeveloperID.p12
```

등록 후 다음 태그 릴리스부터 dmg 가 서명 · 공증되어 경고 없이 열립니다.

## 로컬 패키징

```bash
scripts/package-macos.sh 0.2.0   # macOS: dist/SMPX_0.2.0_macos_universal.dmg (서명 변수 없으면 서명 없음)
scripts/package-linux.sh 0.2.0   # Linux(Ubuntu 22.04+): dist/*.deb, dist/*.tar.gz
```
