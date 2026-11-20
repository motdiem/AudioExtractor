#!/usr/bin/env bash
# Checks what `wails3 task release` built, then prints the SHA-256 checksums.
# Any failed check stops the release with a non-zero status.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

ok() { printf '  ok   %s\n' "$*"; }
need() { [[ -e "$1" ]] || die "missing $1"; }

# Unzip into a scratch folder that is always removed.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

log "Verifying the macOS archive"
need "${DIST}/${APP_NAME}-mac.zip"
mkdir "$tmp/mac"
ditto -x -k "${DIST}/${APP_NAME}-mac.zip" "$tmp/mac"
app="$tmp/mac/${APP_NAME}.app"
need "$app"
codesign --verify --deep --strict "$app" || die "codesign verification failed"
ok "codesign --verify --deep --strict"
archs="$(lipo -archs "$app/Contents/MacOS/${APP_NAME}")"
[[ " $archs " == *" arm64 "* && " $archs " == *" x86_64 "* ]] || die "expected arm64 and x86_64, got: $archs"
ok "lipo -archs: $archs"
need "$app/Contents/Resources/THIRD_PARTY_NOTICES.txt"
need "$app/Contents/Resources/licenses/ffmpeg/COPYING.LGPLv2.1"
need "$app/Contents/Resources/licenses/lame/COPYING"
ok "notice and licence texts in Contents/Resources"
for f in README-EN.txt LISEZ-MOI-FR.txt THIRD_PARTY_NOTICES.txt; do need "$tmp/mac/$f"; done
ok "README-EN.txt, LISEZ-MOI-FR.txt and THIRD_PARTY_NOTICES.txt next to the app"
grep -qa -F "$VERSION" "$app/Contents/MacOS/${APP_NAME}" || die "version ${VERSION} not found in the mac binary"
ok "version ${VERSION} stamped in the binary"
# The notice is also embedded in the binary (the Licences link).
grep -qa -F "FFmpeg 9." "$app/Contents/MacOS/${APP_NAME}" || die "notice not embedded in the mac binary"
ok "notice embedded in the binary"

log "Verifying the Windows artifacts"
exe="${DIST}/${APP_NAME}.exe"
need "$exe"
need "${DIST}/${APP_NAME}-windows.zip"
ok "$(python3 -I "${REPO_ROOT}/scripts/release/pe_check.py" "$exe")"
file "$exe" | grep -q "PE32+ executable.*x86-64" || die "file(1) does not report PE32+ x86-64"
ok "file(1): $(file -b "$exe" | cut -d, -f1-2)"
grep -qa -F "$VERSION" "$exe" || die "version ${VERSION} not found in the exe"
ok "version ${VERSION} stamped in the exe"
# Go stores the embedded files as raw bytes.
grep -qa -F "FFmpeg 9." "$exe" || die "notice not embedded in the exe"
grep -qa -F "GNU LESSER GENERAL PUBLIC LICENSE" "$exe" || die "LGPL text not embedded in the exe"
ok "notice and LGPL text embedded in the exe"
mkdir "$tmp/win"
unzip -q "${DIST}/${APP_NAME}-windows.zip" -d "$tmp/win"
for f in "${APP_NAME}.exe" README-EN.txt LISEZ-MOI-FR.txt THIRD_PARTY_NOTICES.txt; do need "$tmp/win/$f"; done
cmp -s "$tmp/win/${APP_NAME}.exe" "$exe" || die "the exe in the zip differs from dist/${APP_NAME}.exe"
ok "zip holds the exe, both READMEs and THIRD_PARTY_NOTICES.txt"

log "Artifacts (version ${VERSION})"
cd "$DIST"
for f in "${APP_NAME}-mac.zip" "${APP_NAME}-windows.zip" "${APP_NAME}.exe"; do
  printf '%6s  ' "$(du -h "$f" | cut -f1)"
  shasum -a 256 "$f"
done
# Keep the checksums next to the artifacts too.
shasum -a 256 "${APP_NAME}-mac.zip" "${APP_NAME}-windows.zip" "${APP_NAME}.exe" > SHA256SUMS.txt
echo
echo "Release OK. Checksums also in dist/SHA256SUMS.txt"
