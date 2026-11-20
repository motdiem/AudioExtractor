package i18n

import (
	"os"
	"os/exec"
)

// A Finder-launched app has no LANG, so ask the system preferences first.
func detect() Lang {
	var tags []string
	for _, key := range []string{"AppleLanguages", "AppleLocale"} {
		if out, err := exec.Command("defaults", "read", "-g", key).Output(); err == nil {
			tags = append(tags, firstTag(string(out)))
		}
	}
	tags = append(tags, os.Getenv("LC_ALL"), os.Getenv("LANG"))
	return FromTags(tags...)
}
