#!/usr/bin/env bash
# Builds bin/AudioExtractor-test.exe: the release exe (same tags and flags as
# `wails3 task windows:build`, ffmpeg embedded) with its own single-instance
# ID, so it can run in the VM even if another copy of the app is open there.
#
#   scripts/windows-test/build-test-exe.sh [--dev]
#
# --dev drops -tags production, so AUDIOEXTRACTOR_FAKE_NO_WEBVIEW2=1 works
# (writes bin/AudioExtractor-dev.exe).
set -euo pipefail
cd "$(dirname "$0")/../.."
tags="-tags production" out=bin/AudioExtractor-test.exe
if [[ "${1:-}" == --dev ]]; then tags="" out=bin/AudioExtractor-dev.exe; fi
env -u GOPATH wails3 task common:build:frontend >/dev/null
env -u GOPATH wails3 task windows:generate:syso ARCH=amd64 >/dev/null
trap 'rm -f ./*.syso' EXIT
env -u GOPATH GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $tags -trimpath -buildvcs=false \
  -ldflags="-w -s -H windowsgui -X main.version=$(git describe --tags --always --dirty)-vmtest -X main.singleInstanceID=com.motdiem.audioextractor.vmtest" -o "$out" .
git checkout -- build/darwin/Assets.car 2>/dev/null || true
ls -l "$out"
