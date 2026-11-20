#!/usr/bin/env bash
# Produces the Windows artifacts in dist/:
#   AudioExtractor.exe          the single-file app (notices embedded in it)
#   AudioExtractor-windows.zip  the exe + first-launch notes + notices: this is
#                               the one to send to people.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

exe="${BIN}/${APP_NAME}.exe"
[[ -f "$exe" ]] || die "missing ${exe} (build it with: wails3 task windows:build)"
command -v zip >/dev/null || die "zip not found"

cp "$exe" "${DIST}/${APP_NAME}.exe"

st="${DIST}/.stage-windows"
stage "$st"
cp "$exe" "${st}/${APP_NAME}.exe"
cp "${NOTES}/windows/README-EN.txt" "${NOTES}/windows/LISEZ-MOI-FR.txt" "$NOTICE" "$st/"

log "dist/${APP_NAME}-windows.zip"
rm -f "${DIST}/${APP_NAME}-windows.zip"
(cd "$st" && zip -q -X -9 "${DIST}/${APP_NAME}-windows.zip" ./*)
rm -rf "$st"
