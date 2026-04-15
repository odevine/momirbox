package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Preferences struct {
	EnableTokens  bool `json:"enable_tokens"`
	IncludeUnsets bool `json:"include_unsets"`
}

var CurrentPrefs Preferences

// InitPrefs loads user settings from disk or falls back to defaults
func InitPrefs() {
	CurrentPrefs = Preferences{
		EnableTokens:  true,
		IncludeUnsets: false,
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