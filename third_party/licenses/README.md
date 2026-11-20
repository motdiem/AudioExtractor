# Third-party licences

AudioExtractor bundles a slim ffmpeg executable built from unmodified upstream
sources. These texts ship with every release (see ../../THIRD_PARTY_NOTICES.txt).

| Component | Version | Licence | Source |
|-----------|---------|---------|--------|
| FFmpeg | 9.0.2 | LGPL v2.1 or later (built without `--enable-gpl`, `--enable-nonfree` or `--enable-version3`) | https://ffmpeg.org/releases/ffmpeg-9.0.2.tar.xz |
| LAME (libmp3lame) | 3.100 | LGPL v2 or later | https://downloads.sourceforge.net/project/lame/lame/3.100/lame-3.100.tar.gz |

- `ffmpeg/COPYING.LGPLv2.1`, `ffmpeg/LICENSE.md`: copied from the FFmpeg tarball.
- `lame/COPYING`, `lame/LICENSE`: copied from the LAME tarball.

The exact versions, checksums and configure flags are in
`scripts/ffmpeg/config.sh`; `ffmpeg -buildconf` on the bundled binary prints
the full configuration. To rebuild it: `wails3 task ffmpeg:mac`.

When bumping a version in `config.sh`, copy these files again from the new
tarballs (in `.cache/ffmpeg/downloads/`) and update the table above.

## Other components

Also compiled into the app (full texts in the folders here): `wails/` (Wails
v3.1.0, MIT; `wailsio-runtime/` is the same licence, the JavaScript runtime
ships in the same project and has no licence file of its own), `go/` (Go 1.27
standard library, BSD-3), `golang-x-sys/` (BSD-3), `go-ole/` (Windows only,
MIT). Re-check with `go list -deps -f '{{with .Module}}{{.Path}} {{.Version}}{{end}}' .`
(for both GOOS=darwin and GOOS=windows) when dependencies change.

`THIRD_PARTY_NOTICES.txt` at the repo root is the file users see. It is
embedded in the app (`notices.go`), copied into the macOS bundle and placed
next to the Windows exe by `wails3 task release`.
