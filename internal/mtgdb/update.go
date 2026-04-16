package mtgdb

import (
	"context"
	"encoding/json"
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

// MTGSetMeta represents basic set metadata used for synchronization tracking.
type MTGSetMeta struct {
	Code        string `json:"code"`
	ReleaseDate string `json:"releaseDate"`
}

// SetListResponse encapsulates the collection of sets returned by the MTGJSON API.
type SetListResponse struct {
	Data []MTGSetMeta `json:"data"`
}

const (
	SetListURL      = "https://mtgjson.com/api/v5/SetList.json"
	AllPrintingsURL = "https://mtgjson.com/api/v5/AllPrintings.json"
	BulkTimeout     = 15 * time.Minute
	RequestTimeout  = 10 * time.Second
)

// UpdateDatabase synchronizes the local card database with MTGJSON.
// It performs a bulk download if no database exists, otherwise it fetches incremental updates.
func UpdateDatabase(cancelChan <-chan struct{}, callback SyncCallback) {
	callback("Checking DB...", "Contacting MTGJSON...", 0.1, false)

	updatesDir := filepath.Join(config.DataDir, "updates")
	_ = os.MkdirAll(updatesDir, os.ModePerm)

	trackedSetsPath := filepath.Join(config.DataDir, "tracked_sets.json")
	allPrintingsPath := filepath.Join(config.DataDir, "AllPrintings.json")

	setList, err := fetchSetList(cancelChan)
	if err != nil {
		handleUpdateError(err, callback)
		return
	}

	trackedSets := loadTrackedSets(trackedSetsPath)
	missingSets := filterMissingSets(setList.Data, trackedSets)

	if len(missingSets) == 0 {
		completeSync("DB Up To Date!", callback)
		return
	}

	// Scenario: Full Refresh (Base DB missing or tracking is empty)
	if len(trackedSets) == 0 || !FileExists(allPrintingsPath) {
		if err := performBulkDownload(allPrintingsPath, cancelChan, callback); err != nil {
			handleUpdateError(err, callback)
			return
		}
		SaveTrackedSets(trackedSetsPath, setList.Data)
		completeSync("Update Complete!", callback)
		return
	}

	// Scenario: Incremental Catch-up
	if err := performIncrementalUpdate(missingSets, updatesDir, trackedSetsPath, trackedSets, cancelChan, callback); err != nil {
		handleUpdateError(err, callback)
		return
	}

	completeSync("Update Complete!", callback)
}

func fetchSetList(cancelChan <-chan struct{}) (*SetListResponse, error) {
	client := &http.Client{Timeout: RequestTimeout}
	ctx, cancel := CreateCancelContext(cancelChan)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", SetListURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var sl SetListResponse
	if err := json.NewDecoder(resp.Body).Decode(&sl); err != nil {
		return nil, err
	}
	return &sl, nil
}

func performBulkDownload(dest string, cancelChan <-chan struct{}, callback SyncCallback) error {
	callback("Initial Setup", "Downloading bulk DB...", 0.2, false)

	client := &http.Client{Timeout: BulkTimeout}
	ctx, cancel := CreateCancelContext(cancelChan)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", AllPrintingsURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", config.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Use temporary file for atomic transition
	tmpPath := dest + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	pr := &progressReader{
		Reader:     resp.Body,
		Total:      resp.ContentLength,
		Tracker:    NewTracker(float64(resp.ContentLength)),
		Callback:   callback,
		CancelChan: cancelChan,
		LastUpdate: time.Now(),
	}

	_, copyErr := io.Copy(out, pr)
	out.Close()

	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return copyErr
	}

	return os.Rename(tmpPath, dest)
}

func performIncrementalUpdate(codes []string, dir, trackPath string, tracked map[string]bool, cancelChan <-chan struct{}, callback SyncCallback) error {
	client := &http.Client{Timeout: RequestTimeout}

	for i, code := range codes {
		select {
		case <-cancelChan:
			return context.Canceled
		default:
		}

		progress := float64(i+1) / float64(len(codes))
		callback(fmt.Sprintf("Fetching %s...", code), fmt.Sprintf("Set %d of %d", i+1, len(codes)), progress, false)

		if err := downloadSetFile(client, code, dir, cancelChan); err != nil {
			if IsRootCancellation(err) {
				return err
			}
			log.Error().Err(err).Str("set", code).Msg("incremental download failed")
			continue
		}

		tracked[code] = true
		SaveTrackedSetsMap(trackPath, tracked)

		if !wait(100*time.Millisecond, cancelChan) {
			return context.Canceled
		}
	}
	return nil
}

func downloadSetFile(client *http.Client, code, dir string, cancelChan <-chan struct{}) error {
	ctx, cancel := CreateCancelContext(cancelChan)
	defer cancel()

	url := fmt.Sprintf("https://mtgjson.com/api/v5/%s.json", code)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", config.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("set %s returned status %d", code, resp.StatusCode)
	}

	tmpPath := filepath.Join(dir, code+".json.tmp")
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(out, resp.Body)
	out.Close()

	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return copyErr
	}

	return os.Rename(tmpPath, filepath.Join(dir, code+".json"))
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func SaveTrackedSets(path string, sets []MTGSetMeta) {
	codes := make([]string, 0, len(sets))
	for _, s := range sets {
		codes = append(codes, s.Code)
	}
	data, _ := json.MarshalIndent(codes, "", "  ")
	_ = os.WriteFile(path, data, 0644)
}

func SaveTrackedSetsMap(path string, tracked map[string]bool) {
	codes := make([]string, 0, len(tracked))
	for code := range tracked {
		codes = append(codes, code)
	}
	data, _ := json.MarshalIndent(codes, "", "  ")
	_ = os.WriteFile(path, data, 0644)
}

func loadTrackedSets(path string) map[string]bool {
	tracked := make(map[string]bool)
	if data, err := os.ReadFile(path); err == nil {
		var codes []string
		_ = json.Unmarshal(data, &codes)
		for _, c := range codes {
			tracked[c] = true
		}
	}
	return tracked
}

func filterMissingSets(sets []MTGSetMeta, tracked map[string]bool) []string {
	var missing []string
	for _, s := range sets {
		if !tracked[s.Code] {
			missing = append(missing, s.Code)
		}
	}
	return missing
}

func handleUpdateError(err error, callback SyncCallback) {
	if IsRootCancellation(err) {
		log.Info().Msg("update aborted by user")
		callback("", "", 0.0, true)
		return
	}

	log.Error().Err(err).Msg("database update failed")
	callback("Update Failed!", err.Error(), 0.0, false)
	time.Sleep(2 * time.Second)
	callback("", "", 0.0, true)
}

type progressReader struct {
	io.Reader
	Total      int64
	Downloaded int64
	Tracker    *ProgressTracker
	Callback   SyncCallback
	CancelChan <-chan struct{}
	LastUpdate time.Time
}

func (pr *progressReader) Read(p []byte) (int, error) {
	select {
	case <-pr.CancelChan:
		return 0, context.Canceled
	default:
	}

	n, err := pr.Reader.Read(p)
	pr.Downloaded += int64(n)

	if time.Since(pr.LastUpdate) > 100*time.Millisecond || pr.Downloaded == pr.Total {
		pr.updateProgress()
		pr.LastUpdate = time.Now()
	}

	return n, err
}

func (pr *progressReader) updateProgress() {
	mbDown := float64(pr.Downloaded) / (1024 * 1024)
	var progress float64
	var row2 string

	if pr.Total > 0 {
		var etaStr string
		progress, etaStr = pr.Tracker.GetETA(float64(pr.Downloaded))
		shortETA := strings.TrimPrefix(etaStr, "ETA: ")
		row2 = fmt.Sprintf("%.1f MB | %s", mbDown, shortETA)
	} else {
		row2 = fmt.Sprintf("%.1f MB...", mbDown)
		progress = 0.5
	}

	pr.Callback("Downloading DB...", row2, progress, false)
}
