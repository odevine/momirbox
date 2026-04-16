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

	"github.com/rs/zerolog/log"
)

// SyncCallback provides a hook for the UI to receive real-time progress updates.
type SyncCallback func(row1, row2 string, progress float64, isDone bool)

// MissingFile represents a card asset that needs to be fetched from a remote source.
type MissingFile struct {
	Name       string
	Path       string
	Dir        string
	ScryfallID string
	IsBackFace bool
}

const (
	MaxRetries     = 3
	RetryDelay     = 5 * time.Second
	RateLimitDelay = 35 * time.Second
	DiskWritePause = 150 * time.Millisecond
)

// SyncCreatures identifies missing creature images and downloads them from Scryfall.
func SyncCreatures(cancelChan <-chan struct{}, callback SyncCallback) {
	callback("Parsing DB...", "", 0.0, false)

	momirList, err := ParseAllPrintingsCreatures(cancelChan, callback)
	if err != nil {
		handleSyncError(err, callback)
		return
	}

	callback(fmt.Sprintf("Creatures: %d", len(momirList)), "Checking files...", 0.0, false)

	var missing []MissingFile
	for _, card := range momirList {
		cmcDir := filepath.Join(config.CreaturesDir, fmt.Sprintf("%d", card.CMC))
		filePath := filepath.Join(cmcDir, SanitizeForFilename(card.Name)+".jpg")

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			missing = append(missing, MissingFile{
				Name:       card.Name,
				Path:       filePath,
				Dir:        cmcDir,
				ScryfallID: card.ScryfallID,
			})
		}
	}

	if len(missing) == 0 {
		completeSync("All Images Synced!", callback)
		return
	}

	processDownloadQueue(missing, "Syncing Creatures...", cancelChan, callback)
}

// SyncTokens identifies missing token images and handles double-faced variants.
func SyncTokens(cancelChan <-chan struct{}, callback SyncCallback) {
	callback("Parsing DB...", "Tokens...", 0.0, false)

	tokenList, err := ParseAllPrintingsTokens(cancelChan, callback)
	if err != nil {
		handleSyncError(err, callback)
		return
	}

	callback(fmt.Sprintf("Tokens: %d", len(tokenList)), "Checking files...", 0.0, false)

	var missing []MissingFile
	for _, token := range tokenList {
		saveDir := resolveTokenDir(token)
		filePath := filepath.Join(saveDir, SanitizeForFilename(token.Filename)+".jpg")

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			missing = append(missing, MissingFile{
				Name:       token.Filename,
				Path:       filePath,
				Dir:        saveDir,
				ScryfallID: token.ScryfallID,
				IsBackFace: token.IsBackFace,
			})
		}
	}

	if len(missing) == 0 {
		completeSync("All Tokens Synced!", callback)
		return
	}

	processDownloadQueue(missing, "Syncing Tokens...", cancelChan, callback)
}

func processDownloadQueue(queue []MissingFile, label string, cancelChan <-chan struct{}, callback SyncCallback) {
	client := &http.Client{Timeout: 10 * time.Second}
	tracker := NewTracker(float64(len(queue)))

	for i, item := range queue {
		select {
		case <-cancelChan:
			callback("", "", 0.0, true)
			return
		default:
		}

		progress, etaStr := tracker.GetETA(float64(i + 1))
		row2 := fmt.Sprintf("%d/%d | %s", i+1, len(queue), etaStr)
		callback(label, row2, progress, false)

		_ = os.MkdirAll(item.Dir, os.ModePerm)

		if !downloadAsset(client, item, cancelChan) {
			log.Warn().Str("card", item.Name).Msg("failed to acquire asset after retries")
		}

		if !wait(DiskWritePause, cancelChan) {
			callback("", "", 0.0, true)
			return
		}
	}

	completeSync("All Downloads Done!", callback)
}

func downloadAsset(client *http.Client, item MissingFile, cancelChan <-chan struct{}) bool {
	id := item.ScryfallID
	if len(id) < 2 {
		return false
	}

	targetURL := fmt.Sprintf("https://cards.scryfall.io/normal/front/%c/%c/%s.jpg", id[0], id[1], id)
	
	for retries := 0; retries < MaxRetries; retries++ {
		ctx, cancel := CreateCancelContext(cancelChan)
		req, _ := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
		req.Header.Set("User-Agent", config.UserAgent)
		req.Header.Set("Accept", "image/jpeg")

		resp, err := client.Do(req)
		if err != nil {
			cancel()
			if IsRootCancellation(err) {
				return false
			}
			if !wait(RetryDelay, cancelChan) {
				return false
			}
			continue
		}

		if resp.StatusCode == http.StatusOK {
			success := saveToDisk(item.Path, resp.Body)
			resp.Body.Close()
			cancel()
			return success
		}
		
		resp.Body.Close()
		cancel()

		if resp.StatusCode == http.StatusTooManyRequests {
			log.Warn().Str("card", item.Name).Msg("scryfall rate limit triggered")
			if !wait(RateLimitDelay, cancelChan) {
				return false
			}
			continue
		}

		return false
	}
	return false
}

func saveToDisk(path string, body io.Reader) bool {
	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return false
	}

	if _, err := io.Copy(file, body); err != nil {
		file.Close()
		_ = os.Remove(tmpPath)
		return false
	}
	file.Close()

	return os.Rename(tmpPath, path) == nil
}

// SanitizeForFilename strips invalid characters for cross-platform filesystem safety.
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

func handleSyncError(err error, callback SyncCallback) {
	if IsRootCancellation(err) {
		callback("", "", 0.0, true)
		return
	}
	log.Error().Err(err).Msg("sync process failed")
	callback("Sync Failed!", err.Error(), 0.0, false)
	time.Sleep(2 * time.Second)
	callback("", "", 0.0, true)
}

func wait(d time.Duration, cancelChan <-chan struct{}) bool {
	select {
	case <-time.After(d):
		return true
	case <-cancelChan:
		return false
	}
}

func completeSync(msg string, callback SyncCallback) {
	callback(msg, "", 1.0, false)
	time.Sleep(2 * time.Second)
	callback("", "", 1.0, true)
}