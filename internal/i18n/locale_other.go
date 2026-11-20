//go:build !darwin && !windows

package i18n

import "os"

func detect() Lang {
	return FromTags(os.Getenv("LC_ALL"), os.Getenv("LC_MESSAGES"), os.Getenv("LANG"))
}
