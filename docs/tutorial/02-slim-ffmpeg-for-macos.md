# 2. A slim ffmpeg for macOS, embedded in the app

Goal of this chapter: build our own small, LGPL-only ffmpeg as a universal macOS binary, from pinned sources, with one command. Then embed it in the Go executable and make it runnable on the user's Mac, with no installer and no admin rights.

## Why build our own ffmpeg

AudioExtractor does two things with ffmpeg: copy the AAC track out of an MP4 into an M4A, and re-encode audio to MP3. That is a tiny fraction of what ffmpeg can do.

- **Size.** Homebrew's ffmpeg is a 480 KB executable that loads 41 MB of its own libraries and pulls in 93 other formulae (x264, x265, SDL, and more). We can't ship that. A static ffmpeg with everything built in is tens of megabytes. Ours is about 3 MB per architecture.
- **Licence.** A full-featured ffmpeg usually includes GPL parts (x264, for example), which would make the whole app GPL. Built without `--enable-gpl`, ffmpeg is LGPL. LAME, the MP3 encoder, is LGPL too. Shipping an unmodified LGPL executable next to our code, with its licence and source location, is straightforward.
- **No surprises.** `--disable-autodetect` means the build doesn't silently pick up whatever libraries are installed on the build machine. The binary depends only on macOS system frameworks.
- **Reproducible.** The source versions and SHA-256 checksums are pinned in the repo, so anyone can rebuild the same thing, and upgrading is a one-line change.

## Layout

```
scripts/ffmpeg/
  config.sh        versions, URLs, checksums, configure flags (shared with Windows)
  lib.sh           download + verify, build LAME, build ffmpeg, licence check
  build-mac.sh     macOS: arm64 + x86_64, lipo, ad-hoc sign
  Taskfile.yml     ffmpeg:mac, ffmpeg:smoke (included from the root Taskfile)
internal/ffmpeg/
  ffmpeg.go                 Path(): unpack to the user cache, reuse, atomic install
  embed_darwin.go           ad-hoc signing at unpack time
  embed_darwin_arm64.go     //go:embed bin/darwin/arm64
  embed_darwin_amd64.go     //go:embed bin/darwin/amd64
  embed_windows.go          placeholder until the Windows chapter
  bin/darwin/ffmpeg         the universal binary (git-ignored)
  bin/darwin/{arm64,amd64}/ thin slices that get embedded (git-ignored)
third_party/licenses/   FFmpeg and LAME licence texts
```

`config.sh` holds everything that defines the build. The Windows build (next chapter) sources the same file, so the two platforms can't drift apart.

## Prerequisites

Xcode command line tools, plus two Homebrew tools:

```sh
brew install nasm pkg-config
```

`nasm` assembles ffmpeg's x86 SIMD code for the Intel half of the universal binary. Without it, configure fails for x86_64 (or you'd have to pass `--disable-x86asm` and lose a lot of speed).

## Pinned sources

```sh
FFMPEG_VERSION="9.0.2"
FFMPEG_URL="https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz"
FFMPEG_SHA256="8c3850283eb25fa026482078a04051e0be17347b09ef81a0849bec15a96e002e"

LAME_VERSION="3.100"
LAME_URL="https://downloads.sourceforge.net/project/lame/lame/${LAME_VERSION}/lame-${LAME_VERSION}.tar.gz"
LAME_SHA256="ddfe36cab873794038ae2c1210557ad34857a4b6bdc515785d1da9e175b1da1e"
```

Why 9.0.2 and not 9.1? 9.1 came out the day we did this. 9.0.2 is the second bug-fix release of the previous branch, which is the safer pick for something we build once and then forget. LAME 3.100 (2017) is still the latest LAME release.

The script downloads into `.cache/ffmpeg/downloads/` (git-ignored), checks the SHA-256 and refuses to continue on a mismatch. A cached, verified tarball is not downloaded again.

## The configure flags, explained

The idea: switch everything off, then switch on only what we use.

```sh
--disable-everything      # no codecs, formats, protocols, filters... at all
--disable-autodetect      # don't link whatever happens to be on this Mac
--disable-doc --disable-debug --disable-network
--disable-programs --enable-ffmpeg   # only the ffmpeg CLI (no ffprobe, no ffplay)
--disable-shared --enable-static     # one self-contained executable
--disable-avdevice --disable-swscale # no capture devices, no video scaling
```

Then the allow-list, one line per concern:

| Flag | Why |
|------|-----|
| `--enable-protocol=file,pipe` | Read and write local files. `pipe` is for `-progress pipe:1`, which is how the app will get machine-readable progress. |
| `--enable-demuxer=mov,matroska,mp3,aac,wav` | `mov` reads MP4/MOV/M4A (Zoom recordings). `matroska` reads MKV **and WebM**: there's no separate WebM demuxer. |
| `--enable-decoder=aac,mp3,mp3float,opus` and `'pcm_*'` | Decoding is only needed when we re-encode: AAC/MP3/Opus/PCM sources to MP3, or a non-AAC source to M4A. Configure accepts shell-style wildcards. |
| `--enable-parser=aac,mpegaudio,opus` | Parsers split raw streams into packets (needed for `.aac` and `.mp3` inputs). |
| `--enable-bsf=aac_adtstoasc` | A raw `.aac` file (ADTS) needs this bitstream filter to go into an MP4 container. The muxer inserts it automatically. |
| `--enable-encoder=aac,libmp3lame,pcm_s16le` + `--enable-libmp3lame` | ffmpeg's native AAC encoder (fallback when the source isn't AAC) and MP3 through LAME. `pcm_s16le` is explained in the pitfalls. |
| `--enable-muxer=ipod,mp4,mp3,null` | `ipod` writes `.m4a`, `mp4` for completeness, `mp3`. `null` lets the app decode a file without writing anything, to test whether it's readable. |
| `--enable-filter=aresample,aformat,anull,atrim,abuffer,abuffersink` | The ffmpeg CLI always runs audio through a filter graph, even for plain re-encoding: buffer source/sink at the ends, `aformat` + `aresample` to convert sample formats and channel layouts, `anull` as a pass-through, `atrim` for `-ss`/`-t`. |

There's no ffprobe. `ffmpeg -i file` prints the same stream information (duration, codecs, channels) on stderr, so the app can parse that and we ship a single binary.

**Licence guard.** We never pass `--enable-gpl`, `--enable-nonfree` or `--enable-version3`. After the build, the script checks both `ffmpeg -buildconf` and `ffmpeg -L` (which must say "GNU Lesser General Public License") and fails otherwise.

## Building LAME and ffmpeg for each architecture

For each of arm64 and x86_64 the script:

1. extracts a fresh LAME tree and builds a static `libmp3lame.a` into a per-arch prefix:

   ```sh
   CC="clang -arch x86_64" CFLAGS="-arch x86_64 -mmacosx-version-min=13.0 -O2" \
     ./configure --host=x86_64-apple-darwin --prefix=$PREFIX \
       --disable-shared --enable-static --disable-frontend --disable-decoder
   make && make install
   ```

2. extracts a fresh ffmpeg tree and configures it with the shared flags plus the platform ones:

   ```sh
   ./configure "${FFMPEG_CONFIGURE_FLAGS[@]}" \
     --extra-cflags="-I$PREFIX/include" --extra-ldflags="-L$PREFIX/lib" \
     --arch=x86_64 --target-os=darwin --enable-cross-compile \
     --cc="clang -arch x86_64" \
     --extra-cflags="-mmacosx-version-min=13.0" --extra-ldflags="-mmacosx-version-min=13.0"
   make
   ```

The build machine is Apple Silicon, so arm64 is a native build and x86_64 is a cross-compile. `--enable-cross-compile` stops configure from trying to run the test programs it compiles. (They would actually run through Rosetta, but we don't want to depend on that.) `-mmacosx-version-min=13.0` matches the app's minimum macOS. Passing `--extra-cflags` twice is fine: configure appends them.

## lipo: one universal binary

```sh
lipo -create darwin-arm64/ffmpeg darwin-x86_64/ffmpeg -output ffmpeg
strip -x ffmpeg
codesign --force --sign - ffmpeg
```

`lipo` glues the two thin executables into one "fat" file, and macOS runs the matching half. Check it:

```
$ lipo -info internal/ffmpeg/bin/darwin/ffmpeg
Architectures in the fat file: ... are: x86_64 arm64
$ vtool -show-build -arch x86_64 internal/ffmpeg/bin/darwin/ffmpeg | grep minos
    minos 13.0
$ otool -L internal/ffmpeg/bin/darwin/ffmpeg
    /usr/lib/libSystem.B.dylib
    /System/Library/Frameworks/CoreFoundation.framework/...
    /System/Library/Frameworks/CoreVideo.framework/...
    /System/Library/Frameworks/CoreMedia.framework/...
```

Only system libraries, so it runs on any Mac with macOS 13 or later.

The script then cuts the universal file back into thin slices for embedding (`lipo ffmpeg -thin arm64 -output arm64/ffmpeg`, same for x86_64). That sounds circular, but it matters: see "Embedding it in Go" below. Each slice keeps its own signature, because `codesign` signs every architecture inside a fat file separately. You can test the Intel half on an Apple Silicon Mac with `arch -x86_64 ./ffmpeg -version`, which runs it through Rosetta.

## Run it

```sh
wails3 task ffmpeg:mac
```

On a 10-core M-series Mac it takes about 1 min 40 s from scratch (both architectures, LAME included), and prints:

```
Architectures in the fat file: .../ffmpeg are: x86_64 arm64
ffmpeg version 9.0.2 Copyright (c) 2000-2026 the FFmpeg developers
3278848 .../bin/darwin/amd64/ffmpeg
3070208 .../bin/darwin/arm64/ffmpeg
6379776 .../bin/darwin/ffmpeg
```

Run it twice and you get the same SHA-256 for the universal binary: the build is bit-for-bit reproducible in a given checkout.

## Sizes

| Build | Size |
|-------|------|
| Homebrew ffmpeg 8.0.1 (for reference) | 480 KB executable + 41 MB of FFmpeg libraries + 93 dependency formulae |
| Ours, arm64 only, stripped | 3.1 MB |
| Ours, universal (arm64 + x86_64), stripped and signed | **6.1 MB** (6,379,776 bytes) |
| Ours with `--enable-small`, arm64 only | 2.4 MB |
| The released app zip, before ffmpeg | 8.8 MB |
| The released app zip, embedding the universal ffmpeg | 15 MB |
| The released app zip, embedding one thin slice per architecture | **12.3 MB** |

`--enable-small` optimises for size (`-Os`) and drops long descriptive strings. It saves about 22%, but it made ffmpeg's AAC encoder **2.2 times slower** in our test (10 minutes of stereo audio: 5.9 s vs 13.0 s). LAME is a separate library and isn't affected (2.7 s in both cases). We kept the faster build: 1.3 MB doesn't matter in a 10 MB app, and a slow AAC fallback would.

## Should the binary be committed?

No. It's a 6 MB build output that can be regenerated, and every version bump would add 6 MB to the git history. It's git-ignored, and:

- `wails3 task release` calls an internal `ffmpeg:mac:ensure` task first. That task checks `build-mac.sh --is-current`, which passes only when the binary exists **and** was built from the current `config.sh`/`lib.sh`/`build-mac.sh` (a hash of those files is stored next to the binary). Otherwise it rebuilds. So a fresh clone, or a version bump, just works.
- A Go build without the binary still compiles (see below). `ffmpeg.Path()` then returns `ffmpeg.ErrNotBundled`.

## Embedding it in Go

The binary goes into the Go executable with `go:embed`, from files selected by Go's file-name suffixes (`_darwin_arm64.go`, `_darwin_amd64.go`, `_windows.go`).

Our first version embedded the universal ffmpeg on macOS, and the app zip went from 8.8 MB to 15 MB, about twice what we expected. The reason: a universal Wails app is *two* Go executables (arm64 and amd64) joined by `lipo`, and each one embedded the whole universal ffmpeg. The Intel half carried an arm64 ffmpeg it could never run, and the other way round. So each Go architecture now embeds only its own slice:

```go
// embed_darwin_arm64.go
//go:embed bin/darwin/arm64
var binFS embed.FS

const binDir = "bin/darwin/arm64"

// embed_darwin.go
func embeddedBinary() []byte {
	data, err := binFS.ReadFile(binDir + "/ffmpeg")
	if err != nil {
		return nil // not built yet: Path() returns ErrNotBundled
	}
	return data
}
```

That brought the zip down to 12.3 MB.

We embed the **folder**, not `bin/darwin/arm64/ffmpeg`, because `//go:embed` of a file that doesn't exist is a compile error. The folder contains a committed `README.md`, so the pattern always matches something. The Windows file does the same with `bin/windows/ffmpeg.exe`, so cross-compiling the Windows exe keeps working before the Windows ffmpeg exists.

## Unpacking on first use

An embedded file can't be executed directly: it has to be written to disk first. The package has one function:

```go
func Path() (string, error)
```

On first call it writes the binary to:

```
~/Library/Caches/AudioExtractor/ffmpeg-<first 12 hex chars of its SHA-256>/ffmpeg   (macOS)
%LocalAppData%\AudioExtractor\ffmpeg-<sha>\ffmpeg.exe      (Windows, next chapter)
```

- **Per-user cache** (`os.UserCacheDir()`), so no admin rights are needed.
- **Hash in the folder name**, so a new app version with a new ffmpeg never runs a stale copy, and two app versions can coexist. (The arm64 and x86_64 slices have different hashes, so an Intel copy of the app running under Rosetta gets its own folder too.)
- **Atomic.** It writes to a temp file in the same folder, `chmod 0755`, signs it if needed (below), then `rename`s it into place. A crash never leaves a half-written `ffmpeg` behind, and two app instances racing each other both end up with a complete file.
- **Reuse.** After installing, it writes `ffmpeg.sha256` next to the binary with the hash of what was actually installed. On later launches, if the file's hash matches the stamp, it's reused. A corrupted or deleted file is reinstalled.
- **In-process**, the result is memoised behind a mutex, so concurrent calls from goroutines are fine.

Why a separate stamp instead of comparing with the embedded bytes? Because ad-hoc signing modifies the file. If we ever have to sign at runtime, the installed bytes no longer equal the embedded ones, and the copy would be rewritten on every launch.

The tests (`go test ./internal/ffmpeg`) cover all of this with fake binaries in a temp folder: hashed folder name, reuse without rewriting, accepting a copy changed by signing, repairing a corrupted copy or a missing stamp, signing errors, and 16 concurrent installs.

## Ad-hoc signing

On Apple Silicon, the kernel refuses to run code that has no signature at all. An "ad-hoc" signature (`codesign -s -`) is enough: it carries no identity, it just seals the file's contents. It isn't notarisation, and Gatekeeper still treats the app as unsigned. That's the right-click > Open dance from chapter 1. But ffmpeg launched from our app runs.

Two layers:

1. `build-mac.sh` signs the universal binary, so the embedded bytes are already signed and normally nothing needs to happen at runtime.
2. At unpack time, `prepareBinary` runs `codesign --verify` on the temp file and signs it only if that fails. It also removes a `com.apple.quarantine` attribute if one is present.

A darwin-only test takes a copy of `/usr/bin/true`, strips its signature with `codesign --remove-signature`, runs `prepareBinary`, and checks the result verifies and runs.

## Proof it works

```sh
wails3 task ffmpeg:smoke
```

This runs `TestBundledFFmpegRuns`: the real embedded binary is unpacked to the real cache, its signature is verified, and `ffmpeg -version` runs:

```
/Users/you/Library/Caches/AudioExtractor/ffmpeg-8eeefbd6add3/ffmpeg
ffmpeg version 9.0.2 Copyright (c) 2000-2026 the FFmpeg developers
```

The app also does it at startup (`ffmpeg_check.go`, temporary until the extraction feature uses ffmpeg for real), so `wails3 task dev` prints a line like `ffmpeg: ffmpeg version 9.0.2 (.../ffmpeg-8eeefbd6add3/ffmpeg)`. We checked the released `.app` too: launched normally it unpacked the arm64 slice, and launched with `arch -x86_64` it unpacked and ran the x86_64 slice.

We also ran the conversions the app will need, on test clips generated with Homebrew's ffmpeg:

```sh
ffmpeg -i zoom.mp4  -vn -c:a copy out.m4a                   # remux, instant
ffmpeg -i zoom.mp4  -vn -c:a libmp3lame -b:a 128k out.mp3
ffmpeg -i clip.webm -vn -c:a aac out.m4a                     # Opus -> AAC fallback
ffmpeg -i raw.aac   -c:a copy out.m4a                        # needs aac_adtstoasc
ffmpeg -i zoom.mp4  -vn -f null -                            # decode-only check
ffmpeg -progress pipe:1 -nostats -i zoom.mp4 -vn -c:a copy out.m4a
```

All of them worked, and so did the Intel half under Rosetta. `ffmpeg -i` on a video-only file lists only a video stream, and a truncated MP4 fails with `Invalid data found when processing input`. Those are the signals the error handling will rely on.

## Pitfalls we hit

- **macOS's bash 3.2 and empty arrays.** `/usr/bin/env bash` on macOS is still bash 3.2. With `set -u`, expanding an empty array (`"${cross[@]}"`, empty for the native arm64 build) fails with `cross[@]: unbound variable`. The portable spelling is `${cross[@]+"${cross[@]}"}`.
- **`ffmpeg -L` wraps lines.** Our first licence check grepped for "GNU Lesser General Public License" and failed on a perfectly LGPL binary: the text is wrapped after "Public". We grep for "GNU Lesser General Public" instead.
- **The `null` muxer needs an encoder.** `ffmpeg -i x.mp4 -f null -` failed with `Automatic encoder selection failed Default encoder for format null (codec pcm_s16le) is probably disabled`. With `--disable-everything`, even that trivial encoder is gone. Enabling `pcm_s16le` costs almost nothing.
- **There's no "webm" demuxer.** WebM is handled by the `matroska` demuxer (its full name is `matroska,webm`).
- **`go:embed` of a missing file breaks the build** for everyone who hasn't built ffmpeg yet. Embed the folder, which contains a committed README, and treat a missing binary as `ErrNotBundled`.
- **Signing changes the file.** That's why there's a hash stamp instead of a direct comparison with the embedded bytes.
- **A universal app embeds things twice.** See "Embedding it in Go": embed per-architecture slices, not the universal file.
- **Reproducible, but only per checkout path.** Rebuilding in the same folder gives the same bytes. But `ffmpeg -buildconf` embeds the configure command line, including absolute paths to the LAME prefix, so a build in another folder differs by those strings: same sources and flags, different hash.
- **System frameworks show up in `otool -L`** (CoreFoundation, CoreVideo, CoreMedia) even with `--disable-autodetect`: configure adds them on macOS. They're part of every macOS install, so this is harmless.
- **Testing tip: zsh doesn't word-split.** A loop like `for t in "-i a.mp4 out.m4a"; do ffmpeg $t; done` passes the whole string as one argument in zsh (`Unrecognized option 'i a.mp4 out.m4a'`). Run such loops in `bash -c`.

## Result

- `wails3 task ffmpeg:mac` builds a 6.1 MB universal, LGPL-only, ad-hoc signed ffmpeg (plus about 3 MB thin slices) from pinned, checksummed sources into `internal/ffmpeg/bin/darwin/`.
- `wails3 task release` builds it first if it's missing or stale.
- `ffmpeg.Path()` unpacks the matching slice once per version into the user cache and returns a runnable path. The release zip grows from 8.8 MB to 12.3 MB.
- Licence texts are kept in `third_party/licenses/`.

Next chapter: the same ffmpeg, cross-compiled for Windows.
