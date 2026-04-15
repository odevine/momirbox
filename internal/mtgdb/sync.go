package mtgdb

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"momirbox/internal/config"
)

// SyncCallback provides a hook for the UI to receive real-time progress updates.
type SyncCallback func(row1, row2 string, progress float64, isDone bool)

// SyncCreatures identifies missing creature images and downloads them from Scryfall.
// This implements the logic previously found in fetch_creature_images.
func SyncCreatures(callback SyncCallback) {
	callback("Parsing DB...", "", 0.0, false)

	momirList, err := ParseAllPrintingsCreatures()
	if err != nil {
		callback("Parse Failed!", err.Error(), 0.0, false)
		time.Sleep(2 * time.Second)
		callback("", "", 0.0, true)
		return
	}

	callback(fmt.Sprintf("Creatures: %d", len(momirList)), "Checking files...", 0.0, false)

	type MissingFile struct {
		Name       string
		Path       string
		Dir        string
		ScryfallID string 
	}
	var missingFiles []MissingFile

	for _, card := range momirList {
		cmcStr := fmt.Sprintf("%d", card.CMC)
		cmcDir := filepath.Join(config.CreaturesDir, cmcStr)

		safeName := sanitizeForFilename(card.Name) + ".jpg"
		filePath := filepath.Join(cmcDir, safeName)

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			missingFiles = append(missingFiles, MissingFile{
				Name:       card.Name,
				Path:       filePath,
				Dir:        cmcDir,
				ScryfallID: card.ScryfallID,
			})
		}
	}

	totalMissing := len(missingFiles)
	if totalMissing == 0 {
		callback("All Images Synced!", "", 1.0, false)
		time.Sleep(2 * time.Second)
		callback("", "", 1.0, true)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for i, item := range missingFiles {
		progress := float64(i+1) / float64(totalMissing)
		row2 := fmt.Sprintf("Downld: %d/%d", i+1, totalMissing)
		callback("Downloading...", row2, progress, false)

		os.MkdirAll(item.Dir, os.ModePerm)

		targetURL := fmt.Sprintf("https://api.scryfall.com/cards/%s?format=image&version=normal", item.ScryfallID)

		req, _ := http.NewRequest("GET", targetURL, nil)
		req.Header.Set("User-Agent", config.UserAgent)
		req.Header.Set("Accept", "image/jpeg")

		var success bool
		backoff := 35 * time.Second // Mandatory 30s penalty compliance

		for retries := 0; retries < 3; retries++ {
			resp, err := client.Do(req)
			if err == nil {
				if resp.StatusCode == 200 {
					file, _ := os.Create(item.Path)
					io.Copy(file, resp.Body)
					file.Close()
					success = true
					resp.Body.Close()
					break
				} else if resp.StatusCode == 429 {
					fmt.Printf("Hit 429 Penalty Box for %s. Sleeping for %v...\n", item.Name, backoff)
					resp.Body.Close()
					time.Sleep(backoff)
					continue
				}
				// Break on other HTTP errors (like 404 Not Found)
				resp.Body.Close()
				break
			} else {
				fmt.Printf("Network error for %s. Retrying in 5s...\n", item.Name)
				time.Sleep(5 * time.Second)
			}
		}

		// 4. Evaluate our success flag
		if !success {
			fmt.Printf("Skipping %s after failed retries.\n", item.Name)
		}

		// 5. Fast 110ms delay (safely under the 10 reqs/sec limit allowed for the ID endpoint)
		time.Sleep(110 * time.Millisecond)
	}

	callback("All Downloads Done!", "", 1.0, false)
	time.Sleep(2 * time.Second)
	callback("", "", 1.0, true)
}

// SyncTokens identifies missing token images and handles double-faced variants.
// Replaces fetch_token_images with the same filtering and path logic.
func SyncTokens(callback SyncCallback) {
	callback("Parsing DB...", "Tokens...", 0.0, false)

	tokenList, err := ParseAllPrintingsTokens()
	if err != nil {
		callback("Parse Failed!", err.Error(), 0.0, false)
		time.Sleep(2 * time.Second)
		callback("", "", 0.0, true)
		return
	}

	callback(fmt.Sprintf("Tokens: %d", len(tokenList)), "Checking files...", 0.0, false)

	type MissingFile struct {
		Name       string
		Path       string
		Dir        string
		ScryfallID string
		IsBackFace bool
	}
	var missingFiles []MissingFile

	for _, token := range tokenList {
		// Build the complex token directory structures
		var saveDir string
		switch token.Category {
		case "creatures":
			saveDir = filepath.Join(config.TokensDir, "creatures", token.ColorPath, sanitizeForFilename(token.PTPath))
		case "emblems":
			saveDir = filepath.Join(config.TokensDir, "emblems")
		case "artifacts":
			saveDir = filepath.Join(config.TokensDir, "artifacts")
		default:
			saveDir = filepath.Join(config.TokensDir, "helpers")
		}

		safeName := sanitizeForFilename(token.Filename) + ".jpg"
		filePath := filepath.Join(saveDir, safeName)

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			missingFiles = append(missingFiles, MissingFile{
				Name:       token.Filename,
				Path:       filePath,
				Dir:        saveDir,
				ScryfallID: token.ScryfallID,
				IsBackFace: token.IsBackFace,
			})
		}
	}

	totalMissing := len(missingFiles)
	if totalMissing == 0 {
		callback("All Tokens Synced!", "", 1.0, false)
		time.Sleep(2 * time.Second)
		callback("", "", 1.0, true)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for i, item := range missingFiles {
		progress := float64(i+1) / float64(totalMissing)
		row2 := fmt.Sprintf("Downld: %d/%d", i+1, totalMissing)
		callback("Syncing Tokens...", row2, progress, false)

		os.MkdirAll(item.Dir, os.ModePerm)

		// FAST ENDPOINT: Handle double-faced tokens
		faceParam := ""
		if item.IsBackFace {
			faceParam = "&face=back"
		}
		targetURL := fmt.Sprintf("https://api.scryfall.com/cards/%s?format=image&version=normal%s", item.ScryfallID, faceParam)

		req, _ := http.NewRequest("GET", targetURL, nil)
		req.Header.Set("User-Agent", config.UserAgent)
		req.Header.Set("Accept", "image/jpeg")

		var success bool
		backoff := 35 * time.Second

		for retries := 0; retries < 3; retries++ {
			resp, err := client.Do(req)
			if err == nil {
				if resp.StatusCode == 200 {
					file, _ := os.Create(item.Path)
					io.Copy(file, resp.Body)
					file.Close()
					success = true
					resp.Body.Close()
					break
				} else if resp.StatusCode == 429 {
					fmt.Printf("Hit 429 Penalty Box for %s. Sleeping for %v...\n", item.Name, backoff)
					resp.Body.Close()
					time.Sleep(backoff)
					continue
				}
				resp.Body.Close()
				break
			} else {
				fmt.Printf("Network error for %s. Retrying in 5s...\n", item.Name)
				time.Sleep(5 * time.Second)
			}
		}

		if !success {
			fmt.Printf("Skipping token %s after failed retries.\n", item.Name)
		}

		// Strictly enforced 110ms gap (safe for 10 requests/second limit)
		time.Sleep(110 * time.Millisecond)
	}

	callback("All Downloads Done!", "", 1.0, false)
	time.Sleep(2 * time.Second)
	callback("", "", 1.0, true)
}

// sanitizeForFilename strips invalid characters to ensure cross-platform filesystem compatibility.
func sanitizeForFilename(name string) string {
	var builder strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' || r == '_' {
			builder.WriteRune(r)
		}
	}
	return strings.TrimSpace(builder.String())
}