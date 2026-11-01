# 릴리스 / 설치 프로그램

[English](RELEASE.md) | **한국어**

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

릴리스 태그(`v*`)는 보호 규칙으로 삭제·이동할 수 없습니다.

## macOS 최초 실행

공증(notarization)되지 않은 앱은 다른 Mac 에서 처음 열 때 macOS 가 차단합니다. 최초 1회만 아래처럼 허용하면 이후에는 바로 열립니다.

1. `SMPX` 를 `Applications` 로 옮긴 뒤 실행 → "열 수 없음" 안내가 뜨면 **완료**
2. **시스템 설정 → 개인정보 보호 및 보안** → 아래쪽 "SMPX 이(가) 차단되었습니다" 옆 **그래도 열기**
3. 다시 실행 → **열기**

또는 터미널에서 한 번 실행:

```bash
xattr -dr com.apple.quarantine /Applications/smpx.app
```

## macOS 서명 / 공증 설정

| 인증서 | 비용 | 결과 |
|---|---|---|
| 없음 | 무료 | ad-hoc 서명, 다른 Mac 에서 최초 1회 허용 필요 |
| **Apple Development** (현재 사용) | 무료 (Apple ID) | 개발자 서명, 공증 불가 → 다른 Mac 에서 최초 1회 허용 필요 |
| Developer ID Application | Apple Developer Program 연 $99 | 서명 + 공증 → 경고 없이 열림 |

워크플로는 등록된 인증서 중 Developer ID Application 을 우선 사용하고, 없으면 Apple Development 로 서명합니다 (공증 생략).

> ⚠️ 이 저장소는 공개 저장소입니다. 인증서, 암호, Apple ID, Team ID 는 **절대 파일로 커밋하지 말고** Secrets 에만 등록하세요.

| Secret | 값 |
|---|---|
| `MACOS_CERT_P12_BASE64` | 서명 인증서(.p12 — Apple Development 또는 Developer ID Application)를 base64 로 인코딩한 값 |
| `MACOS_CERT_PASSWORD` | .p12 내보낼 때 지정한 암호 |
| `APPLE_ID` | Apple Developer 계정 이메일 |
| `APPLE_TEAM_ID` | Team ID (developer.apple.com → Membership) |
| `APPLE_APP_PASSWORD` | appleid.apple.com 에서 만든 앱 암호 |

### 1. 인증서 만들기

1. Xcode → **설정(Settings)** → **Accounts** → 계정 선택 → 팀 선택 → **Manage Certificates…**
2. 왼쪽 아래 **+** → **Apple Development** (무료) 또는 **Developer ID Application** (유료 Program 가입 시)
3. 확인: 터미널에서 아래 명령에 인증서 이름이 보이면 성공

```bash
security find-identity -v -p codesigning
```

### 2. 인증서를 .p12 파일로 내보내기

1. **키체인 접근(Keychain Access)** 앱 실행 (Spotlight 에서 "키체인")
2. 왼쪽 **기본 키체인 → 로그인** 선택 → 위쪽 탭에서 **내 인증서** 선택
3. `Apple Development: …` (또는 `Developer ID Application: …`) 항목을 우클릭 → **"…" 내보내기**
   (항목 왼쪽 ▶ 를 펼쳐 개인 키가 함께 있는지 확인 — 개인 키가 없으면 내보낼 수 없음)
4. 파일 포맷 **개인 정보 교환(.p12)** → 바탕화면에 `DeveloperID.p12` 로 저장
5. 내보내기 **암호**를 새로 지정 → 이 값이 `MACOS_CERT_PASSWORD`
6. Mac 로그인 암호를 물으면 입력 (키체인 접근 허용)

### 3. GitHub Secrets 등록

> 암호 / 인증서 값은 채팅이나 파일에 붙여넣지 말고, **본인 터미널(터미널.app)** 에서 직접 입력하세요.
> `gh secret set` 은 값을 입력받는 프롬프트를 띄우며, 입력 값은 화면에 표시되지 않습니다.

`APPLE_ID` / `APPLE_TEAM_ID` / `APPLE_APP_PASSWORD` 는 **Developer ID 로 공증할 때만** 필요합니다. Apple Development 인증서만 쓸 때는 인증서와 암호(빈 암호로 내보냈다면 생략)만 등록합니다.

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

등록 후 다음 태그 릴리스부터 dmg 가 서명됩니다. (Developer ID + Apple 계정 정보까지 등록하면 공증되어 경고 없이 열림)

## 로컬 패키징

```bash
scripts/package-macos.sh 0.2.0   # macOS: dist/SMPX_0.2.0_macos_universal.dmg (서명 변수 없으면 서명 없음)
scripts/package-linux.sh 0.2.0   # Linux(Ubuntu 22.04+): dist/*.deb, dist/*.tar.gz
```
