#!/usr/bin/env bash
# =========================
# Linux 패키징: .deb + .tar.gz
# =========================
#
# 사용법: scripts/package-linux.sh <버전>   (예: 0.2.0)
# 빌드 필요 패키지: libgtk-3-dev libwebkit2gtk-4.1-dev (Ubuntu 22.04+)
#
# .deb 는 실행에 필요한 GTK / WebKitGTK / GStreamer 코덱을 의존성으로 선언하므로
# apt 로 설치하면 추가 설정 없이 바로 실행 가능 (yt-dlp 는 앱이 자동 설치)

set -euo pipefail

VERSION="${1:?버전을 입력하세요 (예: 0.2.0)}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

ARCH="$(dpkg --print-architecture)"
DIST="dist"
NAME="smpx_${VERSION}_linux_${ARCH}"

mkdir -p "$DIST"

echo "▶ 빌드 (linux/$ARCH, v$VERSION)"
wails build -clean -tags webkit2_41 -ldflags "-X main.AppVersion=$VERSION"

BIN="build/bin/smpx"

echo "▶ tar.gz"
TAR_DIR="$(mktemp -d)"
trap 'rm -rf "$TAR_DIR" "${PKG_ROOT:-}"' EXIT
mkdir -p "$TAR_DIR/smpx"
install -m755 "$BIN" "$TAR_DIR/smpx/smpx"
install -m644 build/linux/smpx.desktop "$TAR_DIR/smpx/smpx.desktop"
install -m644 build/appicon.png "$TAR_DIR/smpx/smpx.png"
install -m644 LICENSE "$TAR_DIR/smpx/LICENSE"
tar -C "$TAR_DIR" -czf "$DIST/$NAME.tar.gz" smpx

echo "▶ deb"
PKG_ROOT="$(mktemp -d)"
PKG="$PKG_ROOT/smpx"
install -Dm755 "$BIN" "$PKG/usr/bin/smpx"
install -Dm644 build/linux/smpx.desktop "$PKG/usr/share/applications/smpx.desktop"
install -Dm644 build/appicon.png "$PKG/usr/share/pixmaps/smpx.png"
install -Dm644 LICENSE "$PKG/usr/share/doc/smpx/copyright"

INSTALLED_SIZE="$(du -sk "$PKG/usr" | cut -f1)"

mkdir -p "$PKG/DEBIAN"
cat > "$PKG/DEBIAN/control" <<CONTROL
Package: smpx
Version: $VERSION
Section: sound
Priority: optional
Architecture: $ARCH
Installed-Size: $INSTALLED_SIZE
Maintainer: minias <gnuhacker@gmail.com>
Homepage: https://github.com/minias/smpx
Depends: libgtk-3-0 | libgtk-3-0t64, libwebkit2gtk-4.1-0, gstreamer1.0-plugins-base, gstreamer1.0-plugins-good, gstreamer1.0-plugins-bad, gstreamer1.0-libav, ca-certificates
Description: SMPX - YouTube background music player
 YouTube 링크 / 재생목록을 추가해 백그라운드로 재생하는 음악 재생기.
 yt-dlp 는 최초 실행 시 자동으로 설치되고 하루 한 번 업데이트됩니다.
CONTROL

dpkg-deb --build --root-owner-group "$PKG" "$DIST/$NAME.deb"

echo "✔ $DIST/$NAME.deb"
echo "✔ $DIST/$NAME.tar.gz"
