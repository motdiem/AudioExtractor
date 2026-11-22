#!/usr/bin/env bash
# Regenerates the README screenshots in docs/screenshots/: every screen of the
# app, in light and dark, from the dev-only screen previews (frontend/src/preview.ts).
# No Go side and no real files are needed, so the samples stay identical from
# one run to the next.
#
#   bash scripts/screenshots/capture.sh
#
# Needs: frontend/node_modules (npm install in frontend/), Google Chrome, and
# ImageMagick (brew install imagemagick). CHROME overrides the Chrome path.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
out="$root/docs/screenshots"
chrome="${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
# Not 9245, so that `wails3 task dev` can keep running alongside.
port="${SCREENSHOT_PORT:-9246}"
version="$(git -C "$root" describe --tags --abbrev=0 2>/dev/null || echo v0.1.0)"
screens=(idle working done error summary)
# The app window's content size (main.go) and the pixel ratio of the images.
width=420 height=320 scale=2

[ -x "$chrome" ] || { echo "Chrome not found at $chrome (set CHROME)" >&2; exit 1; }
command -v magick >/dev/null || { echo "ImageMagick not found: brew install imagemagick" >&2; exit 1; }

work="$(mktemp -d)"
cleanup() {
	if [ -n "${vite_pid:-}" ]; then
		kill "$vite_pid" 2>/dev/null || true
		wait "$vite_pid" 2>/dev/null || true
	fi
	rm -rf "$work"
}
trap cleanup EXIT

(cd "$root/frontend" && WAILS_VITE_PORT="$port" npx vite --logLevel error) &
vite_pid=$!
for _ in $(seq 1 50); do
	curl -sf -o /dev/null "http://127.0.0.1:$port/" && break
	sleep 0.2
done
curl -sf -o /dev/null "http://127.0.0.1:$port/" || { echo "Vite did not start on port $port" >&2; exit 1; }

mkdir -p "$out"
for scheme in light dark; do
	# Blink's preferredColorScheme: 0 is dark, 1 is light.
	pref=1
	[ "$scheme" = dark ] && pref=0
	for screen in "${screens[@]}"; do
		url="http://127.0.0.1:$port/?screen=$screen&lang=en&os=mac&version=$version"
		# Headless Chrome will not make a window narrower than about 500 px,
		# so the page is laid out in an iframe of the real window size and
		# the screenshot cropped to it.
		cat >"$work/host.html" <<-EOF
			<!doctype html>
			<style>html,body{margin:0}iframe{border:0;display:block;width:${width}px;height:${height}px}</style>
			<iframe src="$url"></iframe>
		EOF
		"$chrome" --headless=new --disable-gpu --hide-scrollbars \
			--user-data-dir="$work/profile" \
			--force-device-scale-factor=$scale --window-size=800,600 \
			--blink-settings=preferredColorScheme=$pref \
			--virtual-time-budget=3000 \
			--screenshot="$work/raw.png" "file://$work/host.html" 2>/dev/null
		# Crop to the window, round the corners, add a hairline border and a
		# soft shadow, so the shot reads as a window on any page background.
		w=$((width * scale)) h=$((height * scale)) r=$((10 * scale))
		border='#00000033'
		[ "$scheme" = dark ] && border='#ffffff33'
		magick "$work/raw.png" -crop "${w}x${h}+0+0" +repage \
			\( -size "${w}x${h}" xc:none -fill white -draw "roundrectangle 0,0,$((w - 1)),$((h - 1)),$r,$r" \) \
			-alpha set -compose DstIn -composite \
			-fill none -stroke "$border" -strokewidth 2 \
			-draw "roundrectangle 1,1,$((w - 2)),$((h - 2)),$r,$r" \
			\( +clone -background black -shadow 30x16+0+8 \) +swap \
			-background none -compose over -layers merge +repage \
			-strip "$out/$screen-$scheme.png"
		echo "$out/$screen-$scheme.png"
	done
done
