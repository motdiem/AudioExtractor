#!/usr/bin/env bash
# Zips the signed universal app with the first-launch notes into
# dist/AudioExtractor-mac.zip. Run by `wails3 task release` after
# `darwin:package:universal`.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

app="${BIN}/${APP_NAME}.app"
[[ -d "$app" ]] || die "missing ${app} (build it with: wails3 task darwin:package:universal)"

st="${DIST}/.stage-mac"
stage "$st"
# ditto keeps the code signature, symlinks and permissions intact.
ditto "$app" "${st}/${APP_NAME}.app"
cp "${NOTES}/mac/README-EN.txt" "${NOTES}/mac/LISEZ-MOI-FR.txt" "$NOTICE" "$st/"

log "dist/${APP_NAME}-mac.zip"
rm -f "${DIST}/${APP_NAME}-mac.zip"
ditto -c -k --norsrc --noextattr "$st" "${DIST}/${APP_NAME}-mac.zip"
rm -rf "$st"
