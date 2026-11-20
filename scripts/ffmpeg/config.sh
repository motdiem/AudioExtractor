# Shared configuration for the slim ffmpeg builds (macOS now, Windows in #4).
#
# This file is sourced by build-mac.sh (and later build-windows.sh). It is the
# single place where source versions, checksums and the ffmpeg feature set live,
# so every platform gets exactly the same ffmpeg.
#
# Bump a version: change *_VERSION, download the tarball, put its SHA-256 here.

FFMPEG_VERSION="9.0.2"
FFMPEG_URL="https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz"
FFMPEG_SHA256="8c3850283eb25fa026482078a04051e0be17347b09ef81a0849bec15a96e002e"

LAME_VERSION="3.100"
LAME_URL="https://downloads.sourceforge.net/project/lame/lame/${LAME_VERSION}/lame-${LAME_VERSION}.tar.gz"
LAME_SHA256="ddfe36cab873794038ae2c1210557ad34857a4b6bdc515785d1da9e175b1da1e"

# LAME: static library only, no command-line frontend, no mp3 decoder (ffmpeg
# has its own). Platform flags (--host, --prefix, CC) are added by the caller.
LAME_CONFIGURE_FLAGS=(
  --disable-shared
  --enable-static
  --disable-frontend
  --disable-decoder
  --disable-gtktest
  --disable-dependency-tracking
)

# ffmpeg: start from nothing, then enable only what AudioExtractor needs.
# Platform flags (--arch, --target-os, --cc, cross-compile, include/lib paths
# for LAME) are added by the caller.
#
# Licence: LGPL v2.1+ only. Never add --enable-gpl, --enable-nonfree or
# --enable-version3 here; build-*.sh checks the result with `ffmpeg -L`.
FFMPEG_CONFIGURE_FLAGS=(
  # Nothing by default, nothing picked up from the build machine.
  --disable-everything
  --disable-autodetect
  --disable-doc
  --disable-debug
  --disable-network

  # Only the ffmpeg command-line tool, statically linked.
  --disable-programs
  --enable-ffmpeg
  --disable-shared
  --enable-static
  --pkg-config-flags=--static

  # Libraries we do not need (avfilter + swresample stay: the CLI needs them).
  --disable-avdevice
  --disable-swscale

  # I/O: local files, plus pipe: for `-progress pipe:1`.
  --enable-protocol=file,pipe

  # Containers we read. The matroska demuxer also handles WebM.
  --enable-demuxer=mov,matroska,mp3,aac,wav

  # Audio decoders (needed for MP3 re-encoding and AAC fallback).
  --enable-decoder=aac,mp3,mp3float,opus
  --enable-decoder='pcm_*'

  # Parsers for the audio streams above.
  --enable-parser=aac,mpegaudio,opus

  # ADTS AAC (raw .aac) -> MP4/M4A needs this bitstream filter.
  --enable-bsf=aac_adtstoasc

  # Encoders: native AAC (fallback when the source is not AAC) and LAME;
  # pcm_s16le is the default audio encoder of the null muxer (decode tests).
  --enable-encoder=aac,libmp3lame,pcm_s16le
  --enable-libmp3lame

  # Containers we write. "null" lets callers decode-test a file (-f null -).
  --enable-muxer=ipod,mp4,mp3,null

  # Filters the ffmpeg CLI inserts in an audio graph: format negotiation,
  # resampling/channel layout, pass-through and trimming for -ss/-t.
  --enable-filter=aresample,aformat,anull,atrim,abuffer,abuffersink
)
