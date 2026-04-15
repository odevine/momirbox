package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// UITheme holds all dynamically adjustable layout values.
type UITheme struct {
	FontSize             int `json:"FontSize"`
	FontDPI              int `json:"FontDPI"`
	SplashTextX          int `json:"SplashTextX"`
	SplashTextY          int `json:"SplashTextY"`
	HeaderTextX          int `json:"HeaderTextX"`
	HeaderTextY          int `json:"HeaderTextY"`
	HeaderLineY1         int `json:"HeaderLineY1"`
	HeaderLineY2         int `json:"HeaderLineY2"`
	VerticalRowHeight    int `json:"VerticalRowHeight"`
	VerticalStartY       int `json:"VerticalStartY"`
	VerticalTextX        int `json:"VerticalTextX"`
	VerticalRightMargin  int `json:"VerticalRightMargin"`
	VerticalMaxVisible   int `json:"VerticalMaxVisible"`
	VerticalHighlightTop int `json:"VerticalHighlightTop"`
	VerticalHighlightBot int `json:"VerticalHighlightBot"`
	CarouselItemSpacing  int `json:"CarouselItemSpacing"`
	CarouselIconSize     int `json:"CarouselIconSize"`
	CarouselIconY        int `json:"CarouselIconY"`
	CarouselTextY        int `json:"CarouselTextY"`
	StatusRow1Y          int `json:"StatusRow1Y"`
	StatusRow2Y          int `json:"StatusRow2Y"`
	ProgressBarX         int `json:"ProgressBarX"`
	ProgressBarY         int `json:"ProgressBarY"`
	ProgressBarWidth     int `json:"ProgressBarWidth"`
	ProgressBarHeight    int `json:"ProgressBarHeight"`
}

// Theme holds the active layout configuration.
var Theme UITheme

// LoadTheme reads the JSON file into the global Theme variable.
func LoadTheme(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &Theme)
}

// WatchTheme runs in the background and reloads the file if it detects a modification.
func WatchTheme(filepath string) {
	var lastModTime time.Time

	for {
		stat, err := os.Stat(filepath)
		if err == nil {
			modTime := stat.ModTime()
			if modTime.After(lastModTime) {
				lastModTime = modTime
				if err := LoadTheme(filepath); err == nil {
					fmt.Println("UI Theme hot-reloaded successfully!")
				} else {
					fmt.Println("Error parsing theme.json:", err)
				}
			}
		}
		// Check for file changes twice a second
		time.Sleep(500 * time.Millisecond)
	}
}