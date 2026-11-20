package window

import "golang.org/x/sys/windows/registry"

// systemDark reads the per-user "app mode" setting (Settings > Personalisation
// > Colours): AppsUseLightTheme is 0 in dark mode.
func systemDark() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && v == 0
}
