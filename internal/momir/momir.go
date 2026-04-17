package momir

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"momirbox/internal/config"
	"momirbox/internal/hardware"
)

// Roll selects a random pre-processed binary file for a given CMC and sends it to the printer.
func Roll(cmc int, printer hardware.Printer) error {
	cmcStr := fmt.Sprintf("%d", cmc)
	cmcDir := filepath.Join(config.CreaturesDir, cmcStr)

	entries, err := os.ReadDir(cmcDir)
	if err != nil {
		return fmt.Errorf("directory not found for CMC %d; ensure images are synced", cmc)
	}

	var validFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(entry.Name())) == ".bin" {
			validFiles = append(validFiles, entry.Name())
		}
	}

	if len(validFiles) == 0 {
		return fmt.Errorf("no valid binary files found for CMC %d", cmc)
	}

	chosenFile := validFiles[rand.Intn(len(validFiles))]
	chosenPath := filepath.Join(cmcDir, chosenFile)
	fmt.Printf("Rolled CMC %d: %s\n", cmc, chosenFile)

	data, err := os.ReadFile(chosenPath)
	if err != nil {
		return fmt.Errorf("failed to read binary file: %w", err)
	}

	return printer.PrintRaw(data)
}

// HasValidImages checks if a given CMC directory exists and contains at least one binary file.
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
		if strings.ToLower(filepath.Ext(entry.Name())) == ".bin" {
			return true
		}
	}
	return false
}
