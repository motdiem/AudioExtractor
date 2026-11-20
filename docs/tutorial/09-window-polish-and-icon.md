# 9. Window polish and a custom icon

Goal of this chapter: make the app feel finished. The window keeps its fixed size and remembers where you put it, follows the system light or dark mode from the first frame, shows its build version, says "Extracteur audio" to French users, and carries its own icon (a waveform coming out of a film frame) on the Mac app, the Dock, the Windows exe and the taskbar.

## The icon

### Design

The brief was "a waveform coming out of a film frame", and the icon has to read at 16 px in a taskbar as well as at 512 px in Finder. Bold, flat geometry survives shrinking, so the design is made only of rectangles with rounded ends, drawn by hand in a 1024 × 1024 SVG (`build/appicon.svg`):

- **Plate**: a full-bleed rounded square with a blue-to-violet diagonal gradient (`#4f6cff` to `#6a35d9`). A saturated mid-tone stays distinct from a white dock and from a near-black taskbar, so one palette works in both themes.
- **Film frame**: white top and bottom strips with three sprocket holes each, closed on the left and **open on the right**. The opening is what makes the waveform "come out".
- **Waveform**: five amber (`#ffc53d`) pill-shaped bars that grow as they leave the frame. Amber on blue-violet is the strongest contrast in the palette, so the sound is what you see first.

Check legibility by rendering small sizes and looking at them, zoomed with nearest-neighbour scaling so you see the real pixels:

```sh
for s in 16 24 32 128; do rsvg-convert -w $s -h $s build/appicon.svg -o /tmp/icon-$s.png; done
magick /tmp/icon-16.png -filter point -resize 1600% /tmp/icon-16-zoom.png
```

At 32 px and up the master is fine. At 16 and 24 px the bars blur into one amber blob, because a 1024-unit shape scaled by 1/64 lands between pixels. The fix is the classic one: **hand-hinted small sizes**. `build/appicon-16.svg` and `build/appicon-24.svg` redraw the same idea on a 16 and 24 unit grid with whole-pixel rectangles (`shape-rendering="crispEdges"`): one-pixel bars with one-pixel gaps, no sprocket holes at 16 px, one-pixel holes at 24 px. They go only into the Windows `.ico`, where those sizes are actually used.

### What each platform wants

| File | Used by | Made from |
| --- | --- | --- |
| `build/appicon.png` (1024²) | input of `wails3 generate icons` | `appicon.svg` shrunk to an 824 px tile with a soft shadow, centred: Apple's icon grid keeps a margin around the plate |
| `build/darwin/icons.icns` | the `.app` before macOS 26 (`CFBundleIconFile`) | `appicon.png`, by `wails3 generate icons` |
| `build/darwin/Assets.car` | the `.app` on macOS 26+, Liquid Glass (`CFBundleIconName`) | `build/appicon.icon` (Icon Composer format), compiled by Xcode's `actool` through `wails3 generate icons -iconcomposerinput` |
| `build/windows/icon.ico` | the `.exe` resources (via `wails3 generate syso`), so Explorer and the taskbar | 16 and 24 px from the hinted SVGs, 32, 48, 64 and 256 px from the master, packed with ImageMagick |

`build/appicon.icon` is a folder: an `icon.json` plus the layers as SVGs. We give it two layers, `Assets/frame.svg` and `Assets/wave.svg` (the master's shapes without the plate), and a background `fill` with an `automatic-gradient` in the plate colour. macOS 26 then draws the glass, the specular highlight and the dark and tinted variants itself. Since `Info.plist` sets `CFBundleIconName` to `appicon`, macOS 26 prefers `Assets.car` over the `.icns`: keeping the Wails sample `.icon` would have left the Wails logo in the Dock even with a new `.icns`.

### One command

```sh
brew install librsvg imagemagick   # rsvg-convert and magick; actool comes with Xcode
wails3 task common:icons:render
git add build && git commit
```

The task renders the master, runs `wails3 generate icons` for the `.icns` and `Assets.car`, then replaces Wails' downscaled `.ico` with crisp per-size renders. To check the macOS result as Finder draws it, ask AppKit for the bundle icon (a five-line Swift script calling `NSWorkspace.shared.icon(forFile:)` on `bin/AudioExtractor.app` does it).

### Why the build used to dirty `Assets.car`

The template's `generate:icons` task ran on every build (`build:native` depends on it) whenever Task thought its sources had changed. Task stores its checksums in `.task/`, which is git-ignored, so a fresh clone, a new worktree or a cleaned tree always re-ran it. That would not matter if the outputs were identical, but `actool` writes a **different `Assets.car` on every run**, even from the same input (the `.icns` and `.ico` come out byte-identical). Hence a modified `Assets.car` after every clean build.

The fix: the icons are build **sources** now, committed like any asset, and `generate:icons` only has a `status:` check that they exist. A normal build says `Task "generate:icons" is up to date` and leaves the tree clean. Regenerating is the explicit `icons:render` task, followed by a commit.

## Light and dark

The page already followed `prefers-color-scheme` through CSS variables. Two native pieces were missing:

- **Title bar.** It follows the system by itself when you do not force it: on macOS leave `MacWindow.Appearance` empty and keep `Backdrop: MacBackdropNormal`; on Windows `WindowsWindow.Theme: application.SystemDefault` (the default, written out to make the intent visible).
- **Window background.** WKWebView and WebView2 show the native background before the page paints. Wails takes one fixed `BackgroundColour` (the zero value unless you set it), which cannot match both themes. `internal/window/theme.go` picks the colour matching the CSS `--bg` (`#ffffff` or `#1e1e20`) and repaints it on the `ThemeChanged` event.

One catch: the window options are built before `app.Run()`, and Wails' `app.Env.IsDarkMode()` returns false until the app runs. So the first colour comes from our own tiny probe: `defaults read -g AppleInterfaceStyle` on macOS (it says `Dark` in dark mode and is absent in light mode), the `AppsUseLightTheme` registry value on Windows.

We considered `MacBackdropTranslucent` instead: the native material follows the theme for free, but it makes the webview transparent and works on macOS only, while the solid colour is one code path for both systems.

## Remembering the position

The settings store from chapter 5 gets a `"window"` key: `{"x": 812, "y": 240}`, the window's top-left corner in desktop coordinates. Wails uses the same space for `Window.Position()` and `Screen.WorkArea`: DIPs (points on macOS), origin at the top-left of the primary screen. That makes the clamp a pure function, `window.Place`, with a table test:

- If the middle of the title bar is on some screen's work area, keep the position, even if part of the window hangs off an edge: the user put it there and can drag it back.
- Otherwise (the external monitor was unplugged, the resolution shrank), move the window to the nearest work area and clamp it fully inside.

The Wails glue (`internal/window/memory.go`):

1. `Restore(&opts, store)` sets `InitialPosition: WindowXY` and `X`, `Y` before the window is created. No saved position means Wails centres it.
2. The screens are only known once the app runs, so `Track` checks the position on the first `WindowRuntimeReady` event and calls `SetPosition` only if `Place` moves it. In the normal case nothing jumps.
3. `WindowDidMove` fires continuously during a drag; the save waits 500 ms after the last one.
4. A pending save is flushed in a `WindowClosing` **hook** (hooks run before the window goes away, listeners run asynchronously and could lose the race with exit) and in `app.OnShutdown` for Cmd+Q. The flushed position is the last one seen in a move event: by then the window may not answer `Position()` any more.

The size stays fixed: `Width = MinWidth = MaxWidth` (same for heights) plus `DisableResize: true`. The window is not always-on-top (the default).

## The version

`main.version` defaults to `dev`. The production builds stamp it from git:

```yaml
# Taskfile.yml
vars:
  VERSION:
    sh: git describe --tags --always --dirty 2>/dev/null || echo dev
```

and the darwin and windows `build:native` tasks add `-X main.version={{.VERSION}}` to their `-ldflags`. We chose `git describe` over a `VERSION` file: nothing to keep in sync, and a build from a modified tree says so (`v1.0.0-3-g29403c8-dirty`). Before the first tag it is just the short hash. To release, tag first: `git tag v1.0.0 && wails3 task release`.

The service exposes it as `Version()`; the idle screen shows it in small muted text in the bottom-right corner, next to the format toggle (ellipsised if long, full text in the tooltip), and Copy details already included it. Dev builds (`wails3 dev`) skip the ldflags and show `dev`.

## Title and subtitle

The window title is always **AudioExtractor**. The catalogs get an `app.subtitle` key, the app's name in each language: "Audio extractor" and "Extracteur audio". The idle screen shows it above the drop zone only when it differs from the title once spaces and case are ignored, so English users see nothing extra and French users see "Extracteur audio". (The catalog test forbids empty messages, which is why English carries a real value instead of "".)

## Layout pass

Nobody had looked at the screens at their real size. The content area is about 420 × 292 on macOS (the title bar takes the rest of the 320) and roughly 404 × 280 on Windows (borders and a taller title bar). The longest strings are the French "unreadable" error (about 200 characters) and a batch summary listing several of them.

To look without the Go side, `frontend/src/preview.ts` renders any screen with sample data from the real catalogs, on the Vite dev server only:

```sh
cd frontend && npx vite --port 9311
# then open http://127.0.0.1:9311/?screen=summary&lang=fr
# screen: idle | working | done | error | summary | stopped
```

`main.ts` loads it behind `if (import.meta.env.DEV)`, so production bundles do not contain it (check: `grep -r hebdomadaire frontend/dist` finds nothing). The dev server needs `server.fs.allow` to read the catalogs from `internal/i18n/locales`.

For screenshots, headless Chrome works, with one trap: it will not make a window narrower than about 500 px, so a 404 px screenshot is a crop of a wider layout. Put each screen in an `<iframe>` of the exact size on one sheet instead, and pass `--blink-settings=preferredColorScheme=0` (dark) or `=1` (light).

What changed in `style.css`:

- `.screen` centres with `justify-content: safe center`: when content is taller than the window it starts at the top instead of overflowing both edges.
- Titles and hints wrap anywhere (`overflow-wrap: anywhere`), for long file names without spaces.
- The error message is its own scroll box (`flex-shrink: 1; overflow-y: auto`), and the error actions sit side by side to save height.
- The failure list takes the remaining height and scrolls, instead of a fixed 96 px.

## Pitfalls

- Never let a build regenerate `Assets.car`: `actool` output is not reproducible.
- With `CFBundleIconName` in `Info.plist`, macOS 26 shows `Assets.car`, not `icons.icns`. Update both.
- `wails3 generate icons` builds the `.ico` by downscaling one PNG; for crisp 16 and 24 px entries, build the `.ico` yourself.
- On macOS `Window.Position()` is top-left based like on Windows (Wails converts from Cocoa's bottom-left origin), so saved positions mean the same thing on both.
- `app.Env.IsDarkMode()` is always false before `app.Run()`.
- Window event listeners run on their own goroutines; use `RegisterHook` for anything that must finish before a window closes.
