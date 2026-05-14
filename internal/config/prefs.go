package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Preferences struct {
	EnableTokens     bool    `json:"enable_tokens"`
	IncludeUnsets    bool    `json:"include_unsets"`
	AnimSpeed        float64 `json:"anim_speed"`
	ScreenTimeoutSec int     `json:"screen_timeout_sec"` // 0 = never
}

var CurrentPrefs Preferences

// InitPrefs loads user settings from disk or falls back to defaults
func InitPrefs() {
	CurrentPrefs = Preferences{
		EnableTokens:     false,
		IncludeUnsets:    false,
		AnimSpeed:        0.15,
		ScreenTimeoutSec: 30,
	}

	if _, err := os.Stat(PrefsFile); err == nil {
		file, err := os.Open(PrefsFile)
		if err == nil {
			defer file.Close()
			json.NewDecoder(file).Decode(&CurrentPrefs)
		}
	}
	// Synchronize the file with current memory state to ensure all keys exist
	SavePrefs()
}

// SavePrefs persists the current configuration to a JSON file
func SavePrefs() {
	os.MkdirAll(filepath.Dir(PrefsFile), os.ModePerm)
	file, err := os.Create(PrefsFile)
	if err != nil {
		fmt.Println("Error saving prefs:", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	encoder.Encode(&CurrentPrefs)
}

func ToggleTokens() {
	CurrentPrefs.EnableTokens = !CurrentPrefs.EnableTokens
	SavePrefs()
}

func ToggleUnsets() {
	CurrentPrefs.IncludeUnsets = !CurrentPrefs.IncludeUnsets
	SavePrefs()
}

// SetScreenTimeout clamps the requested seconds to [0, 300] and persists.
// 0 disables the timeout entirely.
func SetScreenTimeout(sec int) {
	if sec < 0 {
		sec = 0
	}
	if sec > 300 {
		sec = 300
	}
	CurrentPrefs.ScreenTimeoutSec = sec
	SavePrefs()
}
