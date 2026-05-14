package ui

import "momirbox/internal/ui/widget"

// Thin re-exports so callers in cmd/ don't have to import the widget package
// just to bootstrap theme/font loading. Everything else lives in widget.

func LoadTheme(path string) error { return widget.LoadTheme(path) }
func LoadFonts() error            { return widget.LoadFonts() }
func WatchTheme(path string)      { widget.WatchTheme(path) }
