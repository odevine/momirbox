package mtgdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"momirbox/internal/config"
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
func UpdateDatabase(ctx context.Context) error {
	updatesDir := filepath.Join(config.DataDir, "updates")
	_ = os.MkdirAll(updatesDir, os.ModePerm)

	trackedSetsPath := filepath.Join(config.DataDir, "tracked_sets.json")
	allPrintingsPath := filepath.Join(config.DataDir, "AllPrintings.json")

	setList, err := fetchSetList(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch set list: %w", err)
	}

	trackedSets := loadTrackedSets(trackedSetsPath)
	missingSets := filterMissingSets(setList.Data, trackedSets)

	if len(missingSets) == 0 {
		return nil // Up to date
	}

	// Full Refresh
	if len(trackedSets) == 0 || !FileExists(allPrintingsPath) {
		if err := performBulkDownload(ctx, allPrintingsPath); err != nil {
			return fmt.Errorf("bulk download failed: %w", err)
		}
		SaveTrackedSets(trackedSetsPath, setList.Data)
		return nil
	}

	// Incremental Catch-up
	if err := performIncrementalUpdate(ctx, missingSets, updatesDir, trackedSetsPath, trackedSets); err != nil {
		return fmt.Errorf("incremental update failed: %w", err)
	}

	return nil
}

func fetchSetList(ctx context.Context) (*SetListResponse, error) {
	client := &http.Client{Timeout: RequestTimeout}

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

func performBulkDownload(ctx context.Context, dest string) error {
	client := &http.Client{Timeout: BulkTimeout}

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

	tmpPath := dest + ".tmp"
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

	return os.Rename(tmpPath, dest)
}

func performIncrementalUpdate(ctx context.Context, codes []string, dir, trackPath string, tracked map[string]bool) error {
	client := &http.Client{Timeout: RequestTimeout}

	for _, code := range codes {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := downloadSetFile(ctx, client, code, dir); err != nil {
			return err
		}

		tracked[code] = true
		SaveTrackedSetsMap(trackPath, tracked)
		time.Sleep(100 * time.Millisecond) // Play nice with the MTGJSON API
	}
	return nil
}

func downloadSetFile(ctx context.Context, client *http.Client, code, dir string) error {
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
