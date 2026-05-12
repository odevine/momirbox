package mtgdb

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"momirbox/internal/config"
	"momirbox/internal/converter"
)

type MissingFile struct {
	Name       string
	Path       string
	Dir        string
	ScryfallID string
	IsBackFace bool
	Frame      converter.FrameStyle
}

func GetMissingCreatures(ctx context.Context) ([]MissingFile, error) {
	momirList, err := ParseAllPrintingsCreatures(ctx)
	if err != nil {
		return nil, err
	}

	var missing []MissingFile
	for _, card := range momirList {
		cmcDir := filepath.Join(config.CreaturesDir, fmt.Sprintf("%d", card.CMC))
		filePath := filepath.Join(cmcDir, SanitizeForFilename(card.Name)+".bin")

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			missing = append(missing, MissingFile{
				Name:       card.Name,
				Path:       filePath,
				Dir:        cmcDir,
				ScryfallID: card.ScryfallID,
				Frame:      card.Frame,
			})
		}
	}

	return missing, nil
}

func GetMissingTokens(ctx context.Context) ([]MissingFile, error) {
	tokenList, err := ParseAllPrintingsTokens(ctx)
	if err != nil {
		return nil, err
	}

	var missing []MissingFile
	for _, token := range tokenList {
		saveDir := resolveTokenDir(token)
		filePath := filepath.Join(saveDir, SanitizeForFilename(token.Filename)+".bin")

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			missing = append(missing, MissingFile{
				Name:       token.Filename,
				Path:       filePath,
				Dir:        saveDir,
				ScryfallID: token.ScryfallID,
				IsBackFace: token.IsBackFace,
				Frame:      converter.Frame2015,
			})
		}
	}

	return missing, nil
}

func DownloadAndConvert(client *http.Client, item MissingFile, settings converter.DitherSettings, noiseImg image.Image) bool {
	if len(item.ScryfallID) < 2 {
		return false
	}

	targetURL := fmt.Sprintf("https://cards.scryfall.io/large/front/%c/%c/%s.jpg", item.ScryfallID[0], item.ScryfallID[1], item.ScryfallID)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("Accept", "image/jpeg")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return false
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return false
	}

	_ = os.MkdirAll(item.Dir, os.ModePerm)

	frame := item.Frame
	if frame == "" {
		frame = converter.Frame2015
	}
	ditheredGray := converter.DitherImage(img, config.PrinterWidth, settings, frame, noiseImg)
	binData := converter.ImageToESCPOS(ditheredGray)

	tmpPath := item.Path + ".tmp"
	if err := os.WriteFile(tmpPath, binData, 0644); err != nil {
		return false
	}

	return os.Rename(tmpPath, item.Path) == nil
}

func SanitizeForFilename(name string) string {
	var builder strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' || r == '_' {
			builder.WriteRune(r)
		}
	}
	return strings.TrimSpace(builder.String())
}

func resolveTokenDir(token LeanToken) string {
	base := filepath.Join(config.TokensDir, token.Category)
	if token.Category == "creatures" {
		return filepath.Join(base, token.ColorPath, SanitizeForFilename(token.PTPath))
	}
	return base
}
