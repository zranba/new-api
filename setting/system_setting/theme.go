package system_setting

import "github.com/QuantumNous/new-api/setting/config"

type ThemeSettings struct {
	Frontend string `json:"frontend"`
}

var themeSettings = ThemeSettings{
	Frontend: "default",
}

func init() {
	config.GlobalConfig.Register("theme", &themeSettings)
}

func normalizeThemeSettings() {
	themeSettings.Frontend = "default"
}

func GetThemeSettings() *ThemeSettings {
	normalizeThemeSettings()
	return &themeSettings
}

// UpdateAndSyncTheme keeps old theme.frontend values compatible after DB load.
func UpdateAndSyncTheme() {
	normalizeThemeSettings()
}
