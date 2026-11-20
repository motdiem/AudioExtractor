# Helpers shared by the per-platform ffmpeg build scripts. Source, don't run.
#
# Expects: config.sh already sourced, REPO_ROOT set.
# Layout (all git-ignored):
#   .cache/ffmpeg/downloads/   verified source tarballs
#   .cache/ffmpeg/build/<id>/  per-target source trees, LAME prefix, ffmpeg build

set -euo pipefail

CACHE_DIR="${REPO_ROOT}/.cache/ffmpeg"
DOWNLOADS_DIR="${CACHE_DIR}/downloads"
BUILD_ROOT="${CACHE_DIR}/build"
JOBS="$(sysctl -n hw.ncpu 2>/dev/null || nproc 2>/dev/null || echo 4)"

log() { printf '\033[1m==> %s\033[0m\n' "$*" >&2; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

require_tools() {
  local t
  for t in "$@"; do
    command -v "$t" >/dev/null 2>&1 || die "'$t' not found (brew install $t)"
  done
}

# fetch URL SHA256 -> prints the path of the verified tarball.
fetch() {
  local url="$1" sha="$2" file
  file="${DOWNLOADS_DIR}/$(basename "$url")"
  mkdir -p "$DOWNLOADS_DIR"
  if [[ -f "$file" ]] && echo "${sha}  ${file}" | shasum -a 256 -c - >/dev/null 2>&1; then
    echo "$file"; return
  fi
  log "Downloading $(basename "$url")"
  curl -fL --retry 3 -o "${file}.part" "$url"
  echo "${sha}  ${file}.part" | shasum -a 256 -c - >/dev/null \
    || die "checksum mismatch for $(basename "$url") (expected $sha)"
  mv "${file}.part" "$file"
  echo "$file"
}

# extract TARBALL DEST_DIR: fresh source tree with the archive's top folder stripped.
extract() {
  local tarball="$1" dest="$2"
  rm -rf "$dest"; mkdir -p "$dest"
  tar -xf "$tarball" -C "$dest" --strip-components 1
}

# build_lame ID HOST CC CFLAGS -> installs a static libmp3lame into
# ${BUILD_ROOT}/ID/prefix. HOST is an autoconf triplet (e.g. x86_64-apple-darwin).
build_lame() {
  local id="$1" host="$2" cc="$3" cflags="$4"
  local tarball src prefix
  tarball="$(fetch "$LAME_URL" "$LAME_SHA256")"
  src="${BUILD_ROOT}/${id}/lame-src"
  prefix="${BUILD_ROOT}/${id}/prefix"
  log "[$id] LAME ${LAME_VERSION}"
  extract "$tarball" "$src"
  rm -rf "$prefix"
  (
    cd "$src"
    CC="$cc" CFLAGS="$cflags" LDFLAGS="$cflags" \
      ./configure --host="$host" --prefix="$prefix" "${LAME_CONFIGURE_FLAGS[@]}" >"${src}/configure.out" 2>&1 \
      || { tail -30 "${src}/configure.out"; die "LAME configure failed ($id)"; }
    make -j"$JOBS" >"${src}/make.out" 2>&1 || { tail -30 "${src}/make.out"; die "LAME build failed ($id)"; }
    make install >/dev/null
  )
}

# build_ffmpeg ID [platform configure flags...] -> prints the path of the built,
# stripped ffmpeg binary. Links against the LAME built by build_lame for ID.
build_ffmpeg() {
  local id="$1"; shift
  local tarball src prefix
  tarball="$(fetch "$FFMPEG_URL" "$FFMPEG_SHA256")"
  src="${BUILD_ROOT}/${id}/ffmpeg-src"
  prefix="${BUILD_ROOT}/${id}/prefix"
  log "[$id] ffmpeg ${FFMPEG_VERSION}"
  extract "$tarball" "$src"
  (
    cd "$src"
    ./configure \
      --prefix="$prefix" \
      "${FFMPEG_CONFIGURE_FLAGS[@]}" \
      --extra-cflags="-I${prefix}/include" \
      --extra-ldflags="-L${prefix}/lib" \
      "$@" >"${src}/configure.out" 2>&1 \
      || { tail -30 "${src}/configure.out"; tail -30 "${src}/ffbuild/config.log" 2>/dev/null; die "ffmpeg configure failed ($id)"; }
    make -j"$JOBS" >"${src}/make.out" 2>&1 || { tail -40 "${src}/make.out"; die "ffmpeg build failed ($id)"; }
  )
  echo "${src}/ffmpeg"
}

# check_license FFMPEG_BINARY: refuse anything that is not plain LGPL.
# Only works for binaries the host can run.
check_license() {
  local bin="$1" conf
  conf="$("$bin" -hide_banner -buildconf 2>&1)"
  if grep -qE -- '--enable-(gpl|nonfree|version3)' <<<"$conf"; then
    die "$bin was configured with a GPL/nonfree/version3 option"
  fi
  "$bin" -hide_banner -L 2>&1 | grep -q 'GNU Lesser General Public' \
    || die "$bin does not report an LGPL licence"
}
