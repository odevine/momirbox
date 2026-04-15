package config

import (
	"path/filepath"
	"runtime"
	"time"
)

// IsRaspberryPi is determined at runtime based on the operating system and architecture.
var IsRaspberryPi bool

func init() {
	// Identify if we are running on a 32-bit ARM Linux environment (Raspberry Pi Zero 2 W)
	if runtime.GOOS == "linux" && runtime.GOARCH == "arm" {
		IsRaspberryPi = true
	}
}

var (
	BaseDir      = "."
	DataDir      = filepath.Join(BaseDir, "data")
	AssetsDir    = filepath.Join(BaseDir, "assets")
	IconsDir     = filepath.Join(AssetsDir, "icons")
	
	ImagesDir    = filepath.Join(DataDir, "images")
	CreaturesDir = filepath.Join(ImagesDir, "creatures")
	TokensDir    = filepath.Join(ImagesDir, "tokens")
	
	PrefsFile    = filepath.Join(DataDir, "preferences.json")
	LeanDBFile   = filepath.Join(DataDir, "lean_db.json")
)

const (
	CardDataFileName    = "AtomicCards.json"
	PrintingsFileName   = "AllPrintings.json"
	MTGJSONPrintingsURL = "https://mtgjson.com/api/v5/" + PrintingsFileName
	UserAgent           = "MomirBox/1.0"
)

const (
	ScreenWidth  = 128
	ScreenHeight = 64
	PrinterWidth = 384 // standard 58mm thermal paper width in dots
)

const (
	PinBackBtn    = 23
	PinEncoderA   = 17
	PinEncoderB   = 27
	PinEncoderBtn = 22
	// Other navigation buttons will be added here
	PinDisplayDC  = 24
	PinDisplayRST = 25
)

const (
	SplashDuration = 4 * time.Second
	FrameDelay     = 16 * time.Millisecond // targets 60 FPS
	
	HeaderHeight   = 12
	FontSize       = 9
	IconHeight     = 24
	IconWidth      = 24
	ItemSpacing    = 40
	MenuYPos       = 25
)