#!/usr/bin/env bash
# Builds the slim LGPL ffmpeg for macOS as a universal (arm64 + x86_64) binary.
#
#   scripts/ffmpeg/build-mac.sh               # or: wails3 task ffmpeg:mac
#   scripts/ffmpeg/build-mac.sh --is-current  # exit 0 if the binary exists and
#                                             # was built from the current scripts
#
# Output: internal/ffmpeg/bin/darwin/ffmpeg (git-ignored, embedded by Go).
# Needs: Xcode command line tools, nasm (x86 assembly), pkg-config.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "${REPO_ROOT}/scripts/ffmpeg/config.sh"
source "${REPO_ROOT}/scripts/ffmpeg/lib.sh"

MACOS_MIN="13.0"
OUT_DIR="${REPO_ROOT}/internal/ffmpeg/bin/darwin"
export MACOSX_DEPLOYMENT_TARGET="$MACOS_MIN"

# Hash of everything that defines the build. Stored next to the binary (as a
# dot-file, so go:embed ignores it) to detect a stale binary after a version bump.
STAMP="${OUT_DIR}/.build-inputs.sha256"
INPUTS_HASH="$(cd "${REPO_ROOT}/scripts/ffmpeg" && cat config.sh lib.sh build-mac.sh | shasum -a 256 | cut -d' ' -f1)"
if [[ "${1:-}" == "--is-current" ]]; then
  [[ -x "${OUT_DIR}/ffmpeg" && -f "$STAMP" && "$(cat "$STAMP")" == "$INPUTS_HASH" ]]
  exit
fi

[[ "$(uname -s)" == "Darwin" ]] || die "build-mac.sh must run on macOS"
require_tools clang lipo nasm pkg-config make curl shasum

# build_arch ARCH(arm64|x86_64) -> prints the thin ffmpeg path.
build_arch() {
  local arch="$1" id="darwin-$1" cc="clang -arch $1" host ffarch cross=()
  local cflags="-arch ${arch} -mmacosx-version-min=${MACOS_MIN} -O2"
  case "$arch" in
    arm64)  host="aarch64-apple-darwin"; ffarch="aarch64" ;;
    x86_64) host="x86_64-apple-darwin";  ffarch="x86_64" ;;
  esac
  # The host is arm64; x86_64 is a cross-compile (the binary can still be run
  # through Rosetta, but configure must not assume it).
  [[ "$(uname -m)" == "$arch" ]] || cross=(--enable-cross-compile)

  build_lame "$id" "$host" "$cc" "$cflags" >&2
  build_ffmpeg "$id" \
    --arch="$ffarch" --target-os=darwin ${cross[@]+"${cross[@]}"} \
    --cc="$cc" --cxx="clang++ -arch $arch" \
    --extra-cflags="-mmacosx-version-min=${MACOS_MIN}" \
    --extra-ldflags="-mmacosx-version-min=${MACOS_MIN}"
}

arm="$(build_arch arm64)"
x86="$(build_arch x86_64)"

log "lipo -> ${OUT_DIR}/ffmpeg"
mkdir -p "$OUT_DIR"
tmp="${OUT_DIR}/.ffmpeg.tmp"
lipo -create "$arm" "$x86" -output "$tmp"
strip -x "$tmp" 2>/dev/null || strip "$tmp"
# Ad-hoc sign the universal binary so it runs on Apple Silicon straight from
# the build folder. The Go side re-checks the signature after unpacking.
codesign --force --sign - "$tmp"
mv -f "$tmp" "${OUT_DIR}/ffmpeg"

check_license "${OUT_DIR}/ffmpeg"
lipo -info "${OUT_DIR}/ffmpeg"
"${OUT_DIR}/ffmpeg" -hide_banner -version | head -1
ls -lh "${OUT_DIR}/ffmpeg" | awk '{print "size: " $5}'
echo "$INPUTS_HASH" > "$STAMP"
