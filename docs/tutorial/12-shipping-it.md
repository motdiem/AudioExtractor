# 12. Shipping it: licences, release and first-launch notes

Goal of this chapter: turn "it works on my Mac" into two files you can send to someone, with the legal bits in place and a short note that gets a non-technical person past the "unidentified developer" warnings. Everything still runs locally on the Mac, from one command: `wails3 task release`.

## Licences: what we owe, and where it lives

AudioExtractor embeds a slim **ffmpeg** (LGPL v2.1 or later) with **LAME** (LGPL v2 or later) statically linked into it. We build both unmodified from pinned sources, without `--enable-gpl`, `--enable-nonfree` or `--enable-version3`, and the app runs ffmpeg as a separate process. That is the comfortable case, but the LGPL still asks us to:

- give the recipient the licence text,
- name the components and their versions,
- say where the corresponding source can be obtained (and how we built it),
- not stop people replacing the library.

A code review found we had done none of that in the product itself: `third_party/licenses/` held the texts, but nothing packaged them. Now:

- **`THIRD_PARTY_NOTICES.txt`** (repo root) names FFmpeg 9.0.2 and LAME 3.100, states the LGPL terms, lists the download URLs with SHA-256 sums, repeats the exact configure flags from `scripts/ffmpeg/config.sh`, and includes a written offer of the source. It also lists what else is compiled in: Wails and its JavaScript runtime (MIT), the Go standard library and `golang.org/x/sys` (BSD-3), and `go-ole` on Windows (MIT). To find out what really ends up in the binary, ask Go, per platform: `GOOS=windows go list -deps -tags production -f '{{with .Module}}{{.Path}} {{.Version}}{{end}}' .`. Dev-only tools (Vite, TypeScript) are not shipped, so they are not listed.
- **`third_party/licenses/<component>/`** holds the full texts, copied from the module cache or the tarballs.
- **macOS:** the `create:app:bundle` task copies the notice and the licence folders into `AudioExtractor.app/Contents/Resources/` *before* ad-hoc signing, so the signature seals them.
- **Windows is a single exe**, so the texts are embedded in it with `go:embed` (`notices.go` at the repo root, because `go:embed` cannot reach up out of its package folder). A tiny "Licences" link next to the version on the idle screen calls a bound `notices.Service.Open()`, which writes the notice plus every licence text into one file in the temp folder and opens it with the default viewer (`app.Browser.OpenFile`). The same link exists on macOS, which keeps the code path identical.
- Both zips also carry `THIRD_PARTY_NOTICES.txt` next to the app.

The UI change is one `<button class="link">` and two catalog keys (`licences.link`, `licences.title`) in English and French; the catalog test makes sure they stay in step.

## One release task

`wails3 task release` now does, in order:

1. `rm -rf dist`, then `ffmpeg:mac:ensure` and `ffmpeg:windows:ensure` build each ffmpeg only if it is missing or its scripts changed (chapters 2 and 3). From a clean checkout that is the 4 minutes of downloading and compiling.
2. `darwin:package:universal` builds arm64 and amd64, joins them with `lipo`, makes the `.app`, copies in the notices and ad-hoc signs it.
3. `scripts/release/pack-mac.sh` stages the app, the two README files and the notice, and zips them with `ditto -c -k` (the only zip tool that preserves a code signature reliably).
4. `windows:build` cross-compiles the exe (pure Go, `CGO_ENABLED=0`), and `pack-windows.sh` writes two files: `AudioExtractor.exe` on its own, and `AudioExtractor-windows.zip` with the exe, the READMEs and the notice.
5. `scripts/release/verify.sh` checks the result and prints checksums.

**Why a zip for Windows when the goal was "a single exe"?** The exe is still self-contained, and `dist/AudioExtractor.exe` is still produced. But people need the first-launch note (SmartScreen is exactly where non-technical users stop), and an exe cannot carry a README. The zip is the one to send; the bare exe is for you.

The version comes from `git describe --tags --always --dirty` (chapter 9). The release task exports it as `VERSION` so the scripts check the same string the compiler stamped. **Pitfall:** a dirty tree stamps `-dirty`, which is useful: never send such a build.

### What the verification checks

It unzips the Mac archive into a temp folder, so it tests what the recipient gets, not what is in `bin/`:

| Check | How |
|-------|-----|
| Signature | `codesign --verify --deep --strict AudioExtractor.app` |
| Both architectures | `lipo -archs` lists `arm64` and `x86_64` |
| Notices in the bundle | `Contents/Resources/THIRD_PARTY_NOTICES.txt` and `licenses/…` exist |
| Notes in the archive | `README-EN.txt`, `LISEZ-MOI-FR.txt`, notice next to the app |
| Windows exe | `file` says PE32+ x86-64, and a small Python script (`pe_check.py`) walks the resource table to find an icon group, since `file` does not tell you whether the icon is there |
| Notice embedded | the notice text and the LGPL text are found inside both binaries |
| Version | the `git describe` string is found in both binaries |
| Zip contents | the exe in the zip is byte-identical to `dist/AudioExtractor.exe` |

Any failure stops the task with a non-zero exit. The last step prints size and SHA-256 for each artifact and also writes `dist/SHA256SUMS.txt`: send the checksums by another channel (a chat message) and the recipient can verify the download with `shasum -a 256` or `Get-FileHash`.

## First-launch notes

Neither app is signed with a paid identity, so each OS stops a first launch. The notes are plain `.txt` (they open anywhere, even before the app has run), in English and French, one pair per OS because each archive only needs its own: `docs/first-launch/{mac,windows}/README-EN.txt` and `LISEZ-MOI-FR.txt`. They are short and say the same things in the same order: unzip, open the first time, use, where files go, how to remove everything, licences.

- **macOS.** Since macOS 15 (Sequoia), "right-click → Open" no longer bypasses Gatekeeper for an app that is not notarized, and macOS 26 and 27 keep that flow. The path that works: double-click, dismiss the "could not verify" dialog with *Done* (not *Move to Trash*), then **System Settings → Privacy & Security → scroll to Security → Open Anyway**, and authenticate. The note keeps the old right-click route for macOS 14 and earlier, and the `xattr -dr com.apple.quarantine` command as a last resort. The wording of dialogs shifts between releases, so the note describes the steps by their labels without quoting whole sentences.
- **Windows.** SmartScreen's "Windows protected your PC" is a blue window with no visible way forward: **More info → Run anyway**. No installer and no admin rights, so the exe can live in Documents. For locked-down machines, the note mentions *Properties → Unblock*.
- **Where files go**, so nobody is surprised and everything can be removed: the unpacked ffmpeg is in `~/Library/Caches/AudioExtractor` (macOS) or `%LocalAppData%\AudioExtractor` (Windows); settings in `~/Library/Application Support/AudioExtractor` or `%AppData%\AudioExtractor`. These are what `os.UserCacheDir()` and `os.UserConfigDir()` return. Uninstalling is deleting the app and those two folders.

## Pitfalls

- **Sign last.** Anything added to the bundle after `codesign` (a licence file, say) breaks `--verify --deep`. Copy first, sign last. The verification runs on the unzipped copy for the same reason.
- **`go:embed` cannot use `..`.** The notice lives at the repo root, so the embed lives in the root `main` package.
- **Zips and signatures.** `zip` can drop symlinks and extended attributes inside an `.app`; `ditto -c -k` is the right tool for the macOS one. For the Windows one, plain `zip -X` is fine.
- **`dist/` is rebuilt from scratch every time** (`rm -rf dist`), and the release removes the universal binary from `bin/` afterwards so `wails3 task dev` still works (chapter 1).

## Where we are

`wails3 task release` leaves `dist/AudioExtractor-mac.zip`, `dist/AudioExtractor-windows.zip`, `dist/AudioExtractor.exe` and `dist/SHA256SUMS.txt`, verified and checksummed, with the licences inside the apps and the first-launch notes beside them. Updating people is sending a new zip.
