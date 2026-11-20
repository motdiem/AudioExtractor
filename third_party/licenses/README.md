# Third-party licences

AudioExtractor bundles a slim ffmpeg executable built from unmodified upstream
sources. These texts must ship with the app (the release bundling is a later
ticket).

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
