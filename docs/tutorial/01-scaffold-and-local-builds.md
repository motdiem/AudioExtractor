# 1. Scaffold and local builds

Goal of this chapter: a bare **AudioExtractor** window that builds and runs on macOS and Windows, and a single command that produces both distributables, all from one Mac, with no GitHub Actions.

We do the build pipeline first, before any feature. If shipping is painful, you find out on day one, not on the last day.

## Why these choices

- **Wails v3.** A Go backend plus a web frontend rendered in the OS webview (WKWebView on macOS, WebView2 on Windows). No bundled browser, so the app is about 10 MB. v3.1.0 is the stable release.
- **Plain TypeScript, no framework.** The UI is one small window. A framework would be weight with no benefit.
- **Cross-compile from a Mac.** Wails on Windows needs no C compiler (it talks to WebView2 through pure Go), so `GOOS=windows CGO_ENABLED=0` just works from macOS. No Windows machine, no Docker.
- **No CI.** Two people use this tool. A Taskfile that anyone can run locally is the single build entry point, and it is the whole pipeline.

## Prerequisites

Go 1.27, Node, Xcode command line tools, Homebrew. Then install the CLI:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
wails3 doctor
```

`wails3 doctor` ended with `Your system is ready for Wails development!`. The Android, NSIS and Docker lines it lists are optional and we do not need them.

**Pitfall: `GOPATH`.** On this machine `GOPATH` had been set (by mistake) to a copy of `PATH`. `go install` then dropped `wails3` into the first entry of that list followed by `/bin`, which is not on `PATH`, so `wails3: command not found`. Check with `go env GOPATH GOBIN`. Unsetting `GOPATH` (default `~/go`) and adding `~/go/bin` to `PATH` fixed it.

## Scaffold

List the templates:

```sh
wails3 init -l
```

The TypeScript one is simply called `vanilla` (Vanilla + TypeScript + Vite). `vanilla-js` is the JavaScript one.

**Pitfall:** `-t vanilla-ts` is not a template name, and Wails then tries to interpret it as a git URL and fails with `invalid remote git URL`.

```sh
wails3 init -n AudioExtractor -t vanilla -d scaffold \
  -mod github.com/motdiem/AudioExtractor \
  -productidentifier com.motdiem.audioextractor
```

**Pitfall:** `-d` is the parent directory, so the project lands in `scaffold/AudioExtractor/`. We generated it in a temporary directory and copied the contents into the repo root. The `-product*` flags did not reach `build/config.yml`, so we edited it by hand (company, product name, identifier, description, version) and regenerated the platform files:

```sh
wails3 task common:update:build-assets
```

This also recreated the iOS and Linux folders we had deleted, so delete them again. The app targets macOS and Windows only, so we removed `build/ios`, `build/android`, `build/linux`, `build/docker` and their `includes:` lines in `Taskfile.yml`.

Then we stripped the demo down to a bare window: no greet service, no bindings, a 420x320 fixed-size window.

```go
app.Window.NewWithOptions(application.WebviewWindowOptions{
    Title: "AudioExtractor",
    Width: 420, Height: 320,
    MinWidth: 420, MinHeight: 320, MaxWidth: 420, MaxHeight: 320,
    URL: "/",
})
```

The frontend is `frontend/index.html`, `frontend/src/main.ts` and `frontend/public/style.css`. The built output (`frontend/dist`) is embedded into the Go binary with `//go:embed all:frontend/dist`.

## Dev mode

```sh
wails3 task dev     # or: wails3 dev
```

This starts Vite on port 9245, builds the Go app and opens the window pointing at the Vite dev server, with hot reload for the frontend.

**Pitfall: a universal binary in `bin/` breaks dev mode.** After a release build, `bin/AudioExtractor` is a fat (arm64 + x86_64) binary and `go build -o` refuses to overwrite it: `build output "bin/AudioExtractor" already exists and is not an object file`. The release task deletes it at the end.

**Pitfall: an unset background colour shows as black.** On Windows our first test build showed a black window. The page was fine, but the native window behind the webview is black by default and our text was dark on a transparent body. We now set an explicit light/dark `background` and `color` in `style.css`.

## The release task

> **Update:** this is the first, minimal version. By chapter 12 the task also builds both ffmpegs when missing (chapters 2 and 3), bundles the licence notices, adds first-launch notes to each archive, verifies the result and prints checksums. The idea below is unchanged.

`wails3 task` runs the project's Taskfile. The generated platform files already know how to build a universal macOS binary (two `go build`, then `lipo`) and a Windows exe. We only added one task to the root `Taskfile.yml`:

```yaml
release:
  cmds:
    - rm -rf dist
    - mkdir -p dist
    - task: darwin:package:universal
    - ditto -c -k --keepParent "{{.BIN_DIR}}/{{.APP_NAME}}.app" "dist/{{.APP_NAME}}-mac.zip"
    - task: windows:build
      vars: { ARCH: amd64, CGO_ENABLED: "0" }
    - cp "{{.BIN_DIR}}/{{.APP_NAME}}.exe" "dist/{{.APP_NAME}}.exe"
    - rm -f "{{.BIN_DIR}}/{{.APP_NAME}}"
```

Run it:

```sh
wails3 task release
```

It takes about 25 seconds and produces:

```
-rw-r--r--  8.8M  dist/AudioExtractor-mac.zip
-rwxr-xr-x   10M  dist/AudioExtractor.exe
```

Details worth knowing:

- **Ad-hoc signing.** The generated `darwin:create:app:bundle` task already runs `codesign --force --deep --sign -` on the `.app` on a Mac. We did not need to add it. There is no Developer ID, so users must right-click, Open the first time.
- **`ditto`, not `zip`.** `ditto -c -k --keepParent` keeps the bundle's structure and signature intact in the zip.
- **Pitfall: Windows architecture.** The Windows Taskfile defaults `GOARCH` to the host architecture, which is arm64 on Apple Silicon, so a plain `wails3 build GOOS=windows` produced an **ARM64** exe. We pass `ARCH: amd64` explicitly. Check with `file`:

```
dist/AudioExtractor.exe: PE32+ executable (GUI) x86-64, for MS Windows
```

- **macOS minimum version.** The template targets macOS 12, but Go objects were built for 13, giving a linker warning. We set `13.0` in `build/darwin/Taskfile.yml`.
- **Pure Go on Windows.** `CGO_ENABLED=0`, so no C cross toolchain is needed. The `-H windowsgui` linker flag keeps a console window from appearing, and the icon and version info are embedded with a generated `.syso` file.

## Verify

macOS: unzip the artifact the way a user would and check it.

```sh
ditto -x -k dist/AudioExtractor-mac.zip /tmp/check
lipo -info /tmp/check/AudioExtractor.app/Contents/MacOS/AudioExtractor
# Architectures in the fat file: ... are: x86_64 arm64
codesign -dv /tmp/check/AudioExtractor.app   # Signature=adhoc
codesign --verify --deep --strict /tmp/check/AudioExtractor.app
```

Windows, using a Parallels VM from the Mac (the VM is Windows 11 on ARM and runs x64 through emulation). `prlctl exec` can run commands inside the guest, and the Mac home folder is visible as `\\Mac\Home`:

```sh
prlctl resume "Windows 11"
prlctl exec "Windows 11" --current-user powershell -NoProfile -ExecutionPolicy Bypass -File '\\Mac\Home\...\smoke.ps1'
prlctl capture "Windows 11" --file win.png
prlctl suspend "Windows 11"
```

The script copies the exe to the Desktop, starts it with `Start-Process`, and lists the process. It ran as a normal, non-admin user and the window opened.

**Pitfall:** without `--current-user`, `prlctl exec` runs as `SYSTEM`, whose windows you will never see. Another one: a running exe is locked, so a re-copy fails with "being used by another process" until you stop it.

## Result

- `wails3 task dev` runs the window on macOS.
- `wails3 task release` writes `dist/AudioExtractor-mac.zip` (universal, ad-hoc signed) and `dist/AudioExtractor.exe` (x64).
- `dist/`, `bin/`, `node_modules` and `frontend/dist` are git-ignored.

Next chapter: getting a file into the window.
