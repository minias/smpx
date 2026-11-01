#!/usr/bin/env bash

# =========================
# Git Release Automation
# =========================
#
# main 브랜치에서 버전 태그(vX.Y.Z)를 만들어 push 하면
# GitHub Actions(.github/workflows/release.yml)가 macOS dmg / Linux deb·tar.gz 를 빌드해 Release 에 올림

set -euo pipefail

BRANCH="$(git rev-parse --abbrev-ref HEAD)"
echo "현재 브랜치: $BRANCH"

if [ "$BRANCH" != "main" ]; then
  echo "릴리스는 main 브랜치에서만 가능합니다. (dev → main 병합 후 실행)"
  exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "커밋되지 않은 변경사항이 있습니다."
  exit 1
fi

git pull --ff-only origin main

LATEST="$(git tag --list 'v*' --sort=-v:refname | head -1)"
echo "최근 릴리스: ${LATEST:-없음}"

read -r -p "릴리즈 버전 입력 (예: 0.2.0): " VERSION

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "버전 형식이 올바르지 않습니다. (예: 0.2.0)"
  exit 1
fi

TAG="v$VERSION"

if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "이미 존재하는 태그입니다: $TAG"
  exit 1
fi

read -r -p "$TAG 를 만들고 push 할까요? (y/N): " CONFIRM
if [ "$CONFIRM" != "y" ]; then
  echo "취소했습니다."
  exit 0
fi

echo "태그 생성 중..."
git tag -a "$TAG" -m "Release $TAG"

echo "태그 푸시..."
git push origin "$TAG"

echo "릴리즈 시작: $TAG"
echo "진행 상황: https://github.com/minias/smpx/actions/workflows/release.yml"
