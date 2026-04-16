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
	BaseDir   = "."
	DataDir   = filepath.Join(BaseDir, "data")
	AssetsDir = filepath.Join(BaseDir, "assets")
	IconsDir  = filepath.Join(AssetsDir, "icons")

	ImagesDir    = filepath.Join(DataDir, "images")
	CreaturesDir = filepath.Join(ImagesDir, "creatures")
	TokensDir    = filepath.Join(ImagesDir, "tokens")

	PrefsFile  = filepath.Join(DataDir, "preferences.json")
	LeanDBFile = filepath.Join(DataDir, "lean_db.json")
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
	PinBackBtn    = 21
	PinEncoderA   = 20
	PinEncoderB   = 16
	PinEncoderCen = 5
	PinEncoderUp  = 19
	PinEncoderDwn = 6
	PinEncoderLft = 26
	PinEncoderRgt = 13
	PinDisplayDC  = 9
	PinDisplayRST = 27
)

const (
	SplashDuration = 2 * time.Second
	FrameDelay     = 16 * time.Millisecond // targets 60 FPS
)
