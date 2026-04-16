package momir

import (
	"fmt"
	"image"
	_ "image/jpeg" // Register JPEG decoder
	_ "image/png"  // Register PNG decoder
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"momirbox/internal/config"
	"momirbox/internal/hardware"
)

// Roll selects a random card image for a given CMC and sends it to the printer.
// This replaces the original roll_momir and process_and_spool_image functions.
func Roll(cmc int, printer hardware.Printer) error {
	cmcStr := fmt.Sprintf("%d", cmc)
	cmcDir := filepath.Join(config.CreaturesDir, cmcStr)

	// Verify the directory exists for the requested CMC
	entries, err := os.ReadDir(cmcDir)
	if err != nil {
		return fmt.Errorf("directory not found for CMC %d; ensure images are synced", cmc)
	}

	// Filter for supported image formats
	var validImages []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			validImages = append(validImages, entry.Name())
		}
	}

	if len(validImages) == 0 {
		return fmt.Errorf("no valid images found for CMC %d", cmc)
	}

	// Select a random image from the pool
	chosenFile := validImages[rand.Intn(len(validImages))]
	chosenPath := filepath.Join(cmcDir, chosenFile)
	fmt.Printf("Rolled CMC %d: %s\n", cmc, chosenFile)

	file, err := os.Open(chosenPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	// Decode the file into a standard image.Image for processing
	img, _, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Process and output directly via the printer interface
	return printer.PrintImage(img)
}

// HasValidImages checks if a given CMC directory exists and contains at least one image.
func HasValidImages(cmc int) bool {
	cmcStr := fmt.Sprintf("%d", cmc)
	cmcDir := filepath.Join(config.CreaturesDir, cmcStr)

	entries, err := os.ReadDir(cmcDir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			return true // Found at least one valid image!
		}
	}
	return false
}