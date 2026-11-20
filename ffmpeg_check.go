package main

import (
	"log"
	"os/exec"
	"strings"

	"github.com/motdiem/AudioExtractor/internal/ffmpeg"
)

// Startup proof that the bundled ffmpeg unpacks and runs (ticket #3): it logs
// e.g. "ffmpeg: ffmpeg version 9.0.2 (…/ffmpeg-b95614bf5352/ffmpeg)" in the
// `wails3 task dev` console. It also warms the cache so the first extraction
// does not pay for unpacking. Remove it once the extraction service (#5) uses
// ffmpeg.Path() for real.
func init() {
	go func() {
		p, err := ffmpeg.Path()
		if err != nil {
			log.Printf("ffmpeg: %v", err)
			return
		}
		out, err := exec.Command(p, "-hide_banner", "-version").Output()
		if err != nil {
			log.Printf("ffmpeg: %s does not run: %v", p, err)
			return
		}
		first, _, _ := strings.Cut(string(out), "\n")
		version := strings.SplitN(first, " Copyright", 2)[0]
		log.Printf("ffmpeg: %s (%s)", version, p)
	}()
}
