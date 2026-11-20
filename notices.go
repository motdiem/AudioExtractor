package main

import "embed"

// The third-party notices travel inside the binary, so a lone exe copied
// anywhere still carries them (the "Licences" link on the idle screen).
//
//go:embed THIRD_PARTY_NOTICES.txt all:third_party/licenses
var noticeFiles embed.FS
