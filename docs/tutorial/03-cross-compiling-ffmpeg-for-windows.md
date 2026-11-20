# 3. Cross-compiling ffmpeg for Windows, from a Mac

Goal of this chapter: build the same slim, LGPL-only ffmpeg as in chapter 2, but as a Windows x64 `ffmpeg.exe`, on the Mac, with one command. Then embed it in the Windows build of the app and check that it really works on Windows, driving a Windows VM from the Mac's terminal.

No Windows machine is needed to build anything. Windows is only needed to test.

## The plan

- **Same recipe, different compiler.** `config.sh` (versions, checksums, configure flags) and `lib.sh` (download, build LAME, build ffmpeg) are shared with the macOS build. Only a small `build-windows.sh` is new. The two platforms can't drift apart.
- **mingw-w64** is a GCC that produces Windows executables. Homebrew has it, so the Mac can compile for Windows like it compiles for Intel Macs.
- **Fully static.** The exe must not need any mingw DLL (`libgcc_s_seh-1.dll`, `libwinpthread-1.dll`...). Only DLLs that every Windows 10/11 has.
- **Embedded, unpacked on first use** to `%LocalAppData%\AudioExtractor\ffmpeg-<sha>\ffmpeg.exe`, with the Go code from chapter 2. No installer, no admin rights.

## Prerequisites

```sh
brew install mingw-w64 nasm pkg-config
```

That's a big one: Homebrew's mingw-w64 (14.0.0 here, with GCC 16.2) is about 1.4 GB on disk, because it ships compilers for both 32- and 64-bit Windows with their headers and libraries. It gives you `x86_64-w64-mingw32-gcc`, `-strip`, `-objdump` and friends. `nasm` assembles ffmpeg's x86 SIMD code, exactly like for the Intel half of the Mac build.

## The script

`scripts/ffmpeg/build-windows.sh`, in essence:

```sh
source scripts/ffmpeg/config.sh   # pins + FFMPEG_CONFIGURE_FLAGS
source scripts/ffmpeg/lib.sh      # build_lame, build_ffmpeg, ...

HOST=x86_64-w64-mingw32
export SOURCE_DATE_EPOCH=0        # see "Reproducible" below

build_lame windows-x86_64 "$HOST" "${HOST}-gcc" "-O2"
build_ffmpeg windows-x86_64 \
  --arch=x86_64 --target-os=mingw32 --enable-cross-compile \
  --cross-prefix="${HOST}-" \
  --extra-ldflags="-static -static-libgcc -Wl,--no-insert-timestamp"
```

- **LAME** is a classic autoconf project: `--host=x86_64-w64-mingw32` plus `CC=x86_64-w64-mingw32-gcc` is all it takes to cross-compile it into a static `libmp3lame.a`.
- **ffmpeg's configure** has its own cross-compiling flags. `--target-os=mingw32` selects the Windows code paths (native Win32 threads, `CreateProcess`...). `--cross-prefix=x86_64-w64-mingw32-` makes configure use `x86_64-w64-mingw32-gcc`, `-ar`, `-nm`, `-strip` and so on. `--enable-cross-compile` tells it not to try to run what it compiles.
- **`-static -static-libgcc`** links the GCC runtime and everything else into the exe. Without it, a mingw build can depend on `libgcc_s_seh-1.dll` or `libwinpthread-1.dll`, which no Windows user has. ffmpeg uses native Win32 threads on Windows, so winpthreads isn't needed at all.

The output goes to `internal/ffmpeg/bin/windows/ffmpeg.exe` (git-ignored).

```sh
wails3 task ffmpeg:windows
```

It takes about 1 min 20 s on a 10-core M-series Mac from scratch.

## Checking DLL dependencies from the Mac

On a Mac, `otool -L` lists a binary's libraries. For a Windows exe, use mingw's `objdump`:

```
$ x86_64-w64-mingw32-objdump -p internal/ffmpeg/bin/windows/ffmpeg.exe | grep 'DLL Name'
	DLL Name: bcrypt.dll
	DLL Name: KERNEL32.dll
	DLL Name: api-ms-win-crt-conio-l1-1-0.dll
	DLL Name: api-ms-win-crt-convert-l1-1-0.dll
	...
	DLL Name: api-ms-win-crt-utility-l1-1-0.dll
	DLL Name: SHELL32.dll
```

`KERNEL32`, `SHELL32` and `bcrypt` (used for random numbers) are part of Windows. The `api-ms-win-crt-*` names are the **Universal C Runtime** (UCRT): Homebrew's mingw-w64 builds against it rather than the old `msvcrt.dll`. The UCRT is part of Windows 10 and 11, so it's fine for us. On Windows 7 or 8 it would need an update, but the app doesn't target those.

The build script doesn't trust us to check this by hand. It reads the import table and fails if anything outside an allow-list (Windows system DLLs, the UCRT) shows up. If a future change pulls in `libwinpthread-1.dll`, the build stops.

## The licence check, without running the exe

On macOS, the script runs `ffmpeg -L` and `ffmpeg -buildconf` to make sure the build is LGPL. We can't run a Windows exe on the Mac (no Wine here, and we don't want to depend on it). So `lib.sh` has a static variant, `check_license_static`:

- ffmpeg's configure writes `config.h`. It must contain `#define CONFIG_GPL 0`, `CONFIG_NONFREE 0`, `CONFIG_VERSION3 0` and `#define FFMPEG_LICENSE "LGPL version 2.1 or later"`.
- The exe itself must contain the string `LGPL version 2.1 or later` (that's what `ffmpeg -L` prints) and no `--enable-gpl`, `--enable-nonfree` or `--enable-version3` in its embedded configure line.

We then also ran `ffmpeg -L` on Windows (below) to be sure.

## Reproducible

Run the build twice, get the same SHA-256. For the Mac build that came for free. For Windows it didn't: two builds differed by 3 bytes. A PE (Windows executable) header has a **timestamp** field and a checksum computed over the file.

- `-Wl,--no-insert-timestamp` tells the linker to write 0. We checked `ffmpeg_g.exe` (the unstripped exe): its timestamp is 0.
- But ffmpeg's Makefile then runs `strip` to produce `ffmpeg.exe`, and binutils' `strip` **rewrites the PE timestamp with the current time**.
- `strip` honours `SOURCE_DATE_EPOCH`, the standard variable for reproducible builds. Exporting `SOURCE_DATE_EPOCH=0` gives a timestamp of 0 again.

Now two builds in a row give the same file (`7901e20c...` in our checkout). As with the Mac build, the configure line with its absolute paths is compiled in, so a build in another folder differs by those strings.

## One task, kept up to date

`scripts/ffmpeg/Taskfile.yml` gets `ffmpeg:windows` and an internal `ffmpeg:windows:ensure`, next to the macOS ones. `wails3 task release` runs `ffmpeg:windows:ensure` just before building the Windows exe:

```yaml
- task: ffmpeg:mac:ensure
- task: darwin:package:universal
...
- task: ffmpeg:windows:ensure
- task: windows:build
```

`ensure` skips the build when `build-windows.sh --is-current` succeeds: the exe exists and a stamp next to it (`.build-inputs.sha256`) matches the hash of `config.sh`, `lib.sh` and `build-windows.sh`. That logic used to live in `build-mac.sh`. It now sits in `lib.sh` (`stamp_is_current`, `write_stamp`), shared by both scripts. A version bump in `config.sh` therefore rebuilds both platforms on the next release.

## Embedding: nothing to change

Chapter 2 already prepared `embed_windows.go`:

```go
//go:embed bin/windows
var binFS embed.FS

const binaryName = "ffmpeg.exe"
```

The `_windows.go` suffix means it's only compiled for Windows. Once `ffmpeg.exe` is in the folder, the next `GOOS=windows` build embeds it. `Path()` writes it to `os.UserCacheDir()`, which on Windows is `%LocalAppData%`, so the file lands in:

```
C:\Users\<you>\AppData\Local\AudioExtractor\ffmpeg-7901e20cf190\ffmpeg.exe
C:\Users\<you>\AppData\Local\AudioExtractor\ffmpeg-7901e20cf190\ffmpeg.exe.sha256
```

There's nothing to sign and no executable bit on Windows, so `prepareBinary` does nothing. The app already starts ffmpeg with `CREATE_NO_WINDOW`, so no console window pops up.

Sizes:

| | Size |
|-|------|
| `ffmpeg.exe`, static, stripped | **3.5 MB** (3,706,368 bytes; 1.7 MB gzipped) |
| `AudioExtractor.exe` without ffmpeg | 10.6 MB |
| `AudioExtractor.exe` with ffmpeg embedded | **14.3 MB** |

## Testing in a Parallels VM, from the terminal

We test on a Windows 11 VM in Parallels Desktop. On Apple Silicon it's Windows for ARM, which runs x64 programs through emulation, so our x64 exe runs there too. Everything can be done from the Mac's terminal with `prlctl`:

```sh
prlctl list -a                       # is the VM running, stopped, suspended?
prlctl resume "Windows 11"           # wake it up
prlctl exec "Windows 11" --current-user cmd /c ver
prlctl capture "Windows 11" --file /tmp/vm.png   # screenshot
```

- **`--current-user` matters.** Without it the command runs as `SYSTEM`, whose `%LocalAppData%` is somewhere under `C:\Windows\System32\config\systemprofile`. With it, it runs as the logged-in user, which is what we want to test.
- **The Mac home folder is `\\Mac\Home`** inside the VM (Parallels shared folders). So files go in `~/AEWinTest/` on the Mac and are read from `\\Mac\Home\AEWinTest\` on Windows.
- Right after `resume`, `prlctl exec` can fail with "Unable to open new session" for a few seconds. Retry in a loop.

### ffmpeg.exe on its own

Generate a test clip on the Mac with Homebrew's ffmpeg (a 440 Hz tone plus a blue picture, AAC audio):

```sh
ffmpeg -f lavfi -i sine=frequency=440:duration=10 \
       -f lavfi -i color=c=blue:s=320x240:d=10 \
       -c:v mpeg4 -c:a aac -b:a 128k -shortest ~/AEWinTest/test.mp4
```

Then a small batch file, `~/AEWinTest/run.cmd`, copies `ffmpeg.exe` and the clip to a local temp folder and runs:

```bat
ffmpeg.exe -hide_banner -version
ffmpeg.exe -hide_banner -L | findstr /c:"Lesser"
ffmpeg.exe -y -i test.mp4 -vn -c:a copy out.m4a
ffmpeg.exe -y -i test.mp4 -vn -c:a libmp3lame -b:a 128k out.mp3
ffmpeg.exe -y -i test.mp4 -vn -f null -
copy /y out.m4a \\Mac\Home\AEWinTest\
copy /y out.mp3 \\Mac\Home\AEWinTest\
```

```sh
prlctl exec "Windows 11" --current-user cmd /c '\\Mac\Home\AEWinTest\run.cmd'
```

```
ffmpeg version 9.0.2 Copyright (c) 2000-2026 the FFmpeg developers
built with gcc 16.2.0 (GCC)
...
size=     158KiB time=00:00:10.00 bitrate= 129.2kbits/s speed= 682x
size=     157KiB time=00:00:10.00 bitrate= 128.8kbits/s speed=68.5x
```

Back on the Mac, probe what Windows produced:

```
$ ffprobe -v error -show_entries format=format_name,duration,bit_rate:stream=codec_name -of compact out.m4a
stream|codec_name=aac
format|format_name=mov,mp4,m4a,3gp,3g2,mj2|duration=10.000000|bit_rate=129190
$ ffprobe ... out.mp3
stream|codec_name=mp3
format|format_name=mp3|duration=10.000000|bit_rate=128844
```

Both decode cleanly (`ffmpeg -v error -i out.mp3 -f null -` prints nothing). Even under x64 emulation on ARM, the MP3 encode runs at 68 times real time.

### The unpacking code, on real Windows

The Go tests can be cross-compiled into a Windows exe and run in the VM, with no Go installed there:

```sh
GOOS=windows GOARCH=amd64 go test -c -o ~/AEWinTest/ffmpeg.test.exe ./internal/ffmpeg
prlctl exec "Windows 11" --current-user cmd /c '\\Mac\Home\AEWinTest\ffmpeg.test.exe -test.v'
```

`TestBundledFFmpegRuns` unpacked the embedded exe to `C:\Users\ziad\AppData\Local\AudioExtractor\ffmpeg-7901e20cf190\ffmpeg.exe` and ran it. And it found a real bug, see the pitfalls.

### The real app

```sh
wails3 task windows:build ARCH=amd64 CGO_ENABLED=0
```

We copied `bin/AudioExtractor.exe` into the VM, started it with `start`, and took a screenshot with `prlctl capture`: the drop zone and the M4A/MP3 switch, as on the Mac.

Driving an extraction without a mouse took a little more work. The app's UI is a web page inside WebView2, and WebView2 exposes it through **UI Automation**, Windows' accessibility API. From PowerShell:

- find the app's window by process id, then the button with automation id `drop` (our drop zone) and the `M4A`/`MP3` radio buttons. `SelectionItemPattern.Select()` picks the format, `InvokePattern.Invoke()` clicks the drop zone;
- the standard Open dialog appears. Fill its file-name box and press OK;
- the Save dialog appears, opened in the source file's folder. Fill it and press OK;
- wait for the output file.

With M4A and then MP3, the app showed "Done, Saved as app-out.m4a" and wrote the files. The M4A was byte-for-byte identical to the one from running `ffmpeg.exe` directly. Before a third run we deleted `%LocalAppData%\AudioExtractor\ffmpeg-7901e20cf190`, and the app unpacked it again by itself on the next extraction.

## Pitfalls we hit

- **`ffmpeg` vs `ffmpeg.exe`.** `build_ffmpeg` in `lib.sh` returns the path of `ffmpeg`. A mingw build writes `ffmpeg.exe` (and `ffmpeg_g.exe` unstripped). Our first run failed at the copy step.
- **`strip` breaks reproducibility.** See "Reproducible": `--no-insert-timestamp` isn't enough, because `strip` stamps the file again. `SOURCE_DATE_EPOCH=0` fixes it.
- **A rename that works on macOS fails on Windows.** `TestConcurrentInstalls` (16 goroutines installing the same file at once) passed on the Mac and failed in the VM with `rename ... ffmpeg.exe: Access is denied`. On Windows you can't rename onto a file that another thread has open (here: open for hashing), and the winner hadn't written its stamp yet, so the loser gave up. The fix: retry the rename and the stamp write for up to 3 seconds, and accept the other installer's copy as soon as it's valid. 30 runs in a row pass in the VM now. In real life this is two copies of the app starting at the same time on a fresh machine: rare, but it's exactly the kind of bug you only see on the target OS.
- **Quoting through `prlctl exec ... cmd /c`.** Double quotes inside the command came out mangled ("The system cannot find the path specified"). Write a `.cmd` or `.ps1` file in the shared folder and run that: `cmd /c '\\Mac\Home\AEWinTest\run.cmd'`, `powershell -ExecutionPolicy Bypass -File \\Mac\Home\...\x.ps1`.
- **`start` through `prlctl exec` hangs.** Launching a GUI app with `start` from `prlctl exec` opened a visible terminal window in the VM, and the `prlctl` call didn't return. Run it in the background on the Mac side (`prlctl exec ... &`).
- **File dialogs and UI Automation under emulation.** UI Automation saw the web page's buttons fine, but the file dialog's controls showed up as anonymous "panes" without the value or invoke patterns (probably because an ARM64 PowerShell talks to an emulated x64 process). Plain Win32 messages work instead: find the dialog's `Edit` child (inside the combo box with control id 1148 in the Open dialog, id 1001 in the Save dialog), send it `WM_SETTEXT`, then post `WM_COMMAND IDOK` to the dialog. Also, the dialogs are children of the app window in the UI Automation tree, not top-level windows.
- **"Do you want to replace it?"** Our first scripted attempt typed the input path into the Save dialog. Windows asked "test.mp4 already exists. Do you want to replace it?" and nothing was overwritten. Answer it with the task dialog message `TDM_CLICK_BUTTON` and `IDNO`.
- **The extraction tests need a full ffmpeg to make their fixtures.** `go test ./internal/extract` can use a given ffmpeg through `AUDIOEXTRACTOR_FFMPEG`. Pointed at our Windows build, most tests fail before testing anything: they generate test clips with `-f lavfi -i sine=...`, and the slim build has no `lavfi` input. That's expected. The fixtures need a full ffmpeg, the app doesn't.

## Result

- `wails3 task ffmpeg:windows` cross-compiles a 3.5 MB static, LGPL-only `ffmpeg.exe` on the Mac, bit-for-bit reproducibly, from the same pins and flags as the macOS build.
- `wails3 task release` builds it first if it's missing or stale, and the Windows exe embeds it (14.3 MB in total).
- On Windows 11, the app unpacks it to `%LocalAppData%\AudioExtractor\ffmpeg-<sha>\` without admin rights and extracts M4A and MP3.

Next chapter: the first real extraction, drop an MP4 and get an M4A.
