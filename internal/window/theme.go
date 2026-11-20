package window

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// The native background shows before the page paints and around it while it
// loads. It must match --bg in frontend/public/style.css, or a dark-mode
// launch flashes white.
var (
	lightBackground = application.NewRGB(0xff, 0xff, 0xff)
	darkBackground  = application.NewRGB(0x1e, 0x1e, 0x20)
)

// Background returns the native window colour for a theme.
func Background(dark bool) application.RGBA {
	if dark {
		return darkBackground
	}
	return lightBackground
}

// InitialBackground is the background for a window created now, before the
// app runs (Wails can only report the theme once it runs).
func InitialBackground() application.RGBA { return Background(systemDark()) }

// FollowTheme repaints the native background when the system theme changes.
// The title bar follows the theme on its own (macOS appearance left unset,
// Windows Theme: SystemDefault), and the page follows prefers-color-scheme.
func FollowTheme(app *application.App, win *application.WebviewWindow) {
	app.Event.OnApplicationEvent(events.Common.ThemeChanged, func(*application.ApplicationEvent) {
		win.SetBackgroundColour(Background(app.Env.IsDarkMode()))
	})
}
