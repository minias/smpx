#!/usr/bin/env bash
# =========================
# macOS 패키징: 유니버설 .app → (서명) → .dmg → (공증 + staple)
# =========================
#
# 사용법: scripts/package-macos.sh <버전>   (예: 0.2.0)
#
# 서명 / 공증은 환경변수가 있을 때만 수행 (없으면 서명 없는 DMG)
#   MACOS_SIGN_IDENTITY  예: "Developer ID Application: minias lee (TEAMID)"
#   APPLE_ID, APPLE_TEAM_ID, APPLE_APP_PASSWORD  (공증용, 앱 암호)

set -euo pipefail

VERSION="${1:?버전을 입력하세요 (예: 0.2.0)}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

APP="build/bin/smpx.app"
DIST="dist"
DMG="$DIST/SMPX_${VERSION}_macos_universal.dmg"

mkdir -p "$DIST"

echo "▶ 빌드 (darwin/universal, v$VERSION)"
wails build -clean -platform darwin/universal -ldflags "-X main.AppVersion=$VERSION"

# Info.plist 버전 반영
/usr/libexec/PlistBuddy \
  -c "Set :CFBundleShortVersionString $VERSION" \
  -c "Set :CFBundleVersion $VERSION" \
  "$APP/Contents/Info.plist"

if [ -n "${MACOS_SIGN_IDENTITY:-}" ]; then
  echo "▶ 앱 서명 (Hardened Runtime)"
  codesign --force --deep --options runtime --timestamp --sign "$MACOS_SIGN_IDENTITY" "$APP"
  codesign --verify --strict --verbose=2 "$APP"
else
  echo "⚠ MACOS_SIGN_IDENTITY 없음 → 서명 없이 패키징"
fi

echo "▶ DMG 생성 (앱 + Applications 바로가기)"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
cp -R "$APP" "$STAGE/"
ln -s /Applications "$STAGE/Applications"
rm -f "$DMG"
hdiutil create -volname "SMPX $VERSION" -srcfolder "$STAGE" -ov -format UDZO "$DMG"

if [ -n "${MACOS_SIGN_IDENTITY:-}" ]; then
  codesign --force --timestamp --sign "$MACOS_SIGN_IDENTITY" "$DMG"

  if [ -n "${APPLE_ID:-}" ] && [ -n "${APPLE_TEAM_ID:-}" ] && [ -n "${APPLE_APP_PASSWORD:-}" ]; then
    echo "▶ 공증 (notarytool)"
    xcrun notarytool submit "$DMG" \
      --apple-id "$APPLE_ID" \
      --team-id "$APPLE_TEAM_ID" \
      --password "$APPLE_APP_PASSWORD" \
      --wait
    xcrun stapler staple "$DMG"
    spctl --assess --type open --context context:primary-signature --verbose=2 "$DMG"
  else
    echo "⚠ APPLE_ID / APPLE_TEAM_ID / APPLE_APP_PASSWORD 없음 → 공증 생략"
  fi
fi

echo "✔ $DMG"
