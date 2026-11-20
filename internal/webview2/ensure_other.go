//go:build !windows

package webview2

import "github.com/motdiem/AudioExtractor/internal/i18n"

// Ensure always succeeds outside Windows: macOS ships its own WebKit.
func Ensure(i18n.Lang) bool { return true }
