#!/usr/bin/env bash
# Cross-compiles the slim LGPL ffmpeg for Windows x64 on a Mac, with mingw-w64.
#
#   scripts/ffmpeg/build-windows.sh               # or: wails3 task ffmpeg:windows
#   scripts/ffmpeg/build-windows.sh --is-current  # exit 0 if the exe exists and
#                                                 # was built from the current scripts
#
# Output (git-ignored):
#   internal/ffmpeg/bin/windows/ffmpeg.exe   embedded by embed_windows.go
# Needs: mingw-w64 (x86_64-w64-mingw32-gcc), nasm, pkg-config.
#
# Same sources, pins and feature set as the macOS build (config.sh). The exe is
# fully static: besides Windows system DLLs it needs nothing, not even the
# mingw runtime DLLs (libgcc_s, libwinpthread).

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "${REPO_ROOT}/scripts/ffmpeg/config.sh"
source "${REPO_ROOT}/scripts/ffmpeg/lib.sh"

HOST="x86_64-w64-mingw32"
ID="windows-x86_64"
OUT_DIR="${REPO_ROOT}/internal/ffmpeg/bin/windows"
OUT="${OUT_DIR}/ffmpeg.exe"

# See stamp_is_current in lib.sh.
STAMP="${OUT_DIR}/.build-inputs.sha256"
if [[ "${1:-}" == "--is-current" ]]; then
  stamp_is_current "$STAMP" build-windows.sh "$OUT"
  exit
fi

require_tools "${HOST}-gcc" "${HOST}-objdump" nasm pkg-config make curl shasum

# Reproducibility: a PE header has a timestamp field. The linker leaves it at 0
# with --no-insert-timestamp, but `strip` (run by ffmpeg's Makefile) rewrites it
# with the current time unless SOURCE_DATE_EPOCH is set.
LDFLAGS_REPRO="-Wl,--no-insert-timestamp"
export SOURCE_DATE_EPOCH=0

build_lame "$ID" "$HOST" "${HOST}-gcc" "-O2" >&2
ffmpeg_bin="$(build_ffmpeg "$ID" \
  --arch=x86_64 --target-os=mingw32 --enable-cross-compile \
  --cross-prefix="${HOST}-" \
  --extra-ldflags="-static -static-libgcc ${LDFLAGS_REPRO}")"

log "-> ${OUT}"
mkdir -p "$OUT_DIR"
tmp="${OUT_DIR}/.ffmpeg.exe.tmp"
# build_ffmpeg prints the Unix name; the mingw build adds .exe. ffmpeg's
# Makefile has already stripped it (the unstripped one is ffmpeg_g.exe).
cp "${ffmpeg_bin}.exe" "$tmp"
mv -f "$tmp" "$OUT"

check_license_static "${BUILD_ROOT}/${ID}/ffmpeg-src" "$OUT"

# Refuse an exe that needs anything beyond what every Windows install has.
# Homebrew's mingw-w64 targets the Universal C Runtime (api-ms-win-crt-*),
# which ships with Windows 10 and later.
dlls="$("${HOST}-objdump" -p "$OUT" | awk '/DLL Name:/ {print $3}' | sort -u)"
while read -r dll; do
  case "$(tr '[:upper:]' '[:lower:]' <<<"$dll")" in
    kernel32.dll|user32.dll|advapi32.dll|bcrypt.dll|ole32.dll|shell32.dll|ws2_32.dll|msvcrt.dll|api-ms-win-*.dll) ;;
    *) die "ffmpeg.exe depends on a non-system DLL: $dll" ;;
  esac
done <<<"$dlls"

echo "DLLs: $(tr '\n' ' ' <<<"$dlls")"
file "$OUT" 2>/dev/null || true
ls -l "$OUT" | awk '{print $5, $NF}'
shasum -a 256 "$OUT"
write_stamp "$STAMP" build-windows.sh
