//go:build !darwin && !windows

package window

func systemDark() bool { return false }
