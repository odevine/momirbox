package mtgdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"momirbox/internal/config"
)

type MTGSetMeta struct {
	Code        string `json:"code"`
	ReleaseDate string `json:"releaseDate"`
}

type SetListResponse struct {
	Data []MTGSetMeta `json:"data"`
}

// UpdateDatabase fetches SetList.json, compares it to tracked_sets.json, and downloads 
// either the bulk AllPrintings file or just the missing incremental set files.
func UpdateDatabase(callback SyncCallback) {
	callback("Checking DB...", "Contacting MTGJSON...", 0.1, false)

	updatesDir := filepath.Join(config.DataDir, "updates")
	os.MkdirAll(updatesDir, os.ModePerm)
	
	trackedSetsPath := filepath.Join(config.DataDir, "tracked_sets.json")
	allPrintingsPath := filepath.Join(config.DataDir, "AllPrintings.json")

	// 1. Get the master SetList from MTGJSON
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", "https://mtgjson.com/api/v5/SetList.json", nil)
	req.Header.Set("User-Agent", config.UserAgent)

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		callback("Update Failed!", "Failed to fetch SetList.", 0.0, false)
		time.Sleep(2 * time.Second)
		callback("", "", 0.0, true)
		return
	}

	var setList SetListResponse
	if err := json.NewDecoder(resp.Body).Decode(&setList); err != nil {
		callback("Update Failed!", "Invalid SetList JSON.", 0.0, false)
		resp.Body.Close()
		time.Sleep(2 * time.Second)
		callback("", "", 0.0, true)
		return
	}
	resp.Body.Close()

	// 2. Load our currently tracked sets
	trackedSets := make(map[string]bool)
	if data, err := os.ReadFile(trackedSetsPath); err == nil {
		var sets []string
		json.Unmarshal(data, &sets)
		for _, s := range sets {
			trackedSets[s] = true
		}
	}

	// 3. Find out what we are missing
	var missingSets []string
	for _, set := range setList.Data {
		if !trackedSets[set.Code] {
			missingSets = append(missingSets, set.Code)
		}
	}

	if len(missingSets) == 0 {
		callback("DB Up To Date!", "No new sets found.", 1.0, false)
		time.Sleep(2 * time.Second)
		callback("", "", 1.0, true)
		return
	}

	// 4. SCENARIO A: The Big Gulp (First Run)
	if len(trackedSets) == 0 || !fileExists(allPrintingsPath) {
		callback("Initial Setup", "Downloading bulk DB...", 0.2, false)
		
		// 15 minute timeout for the massive 300MB+ file
		bulkClient := &http.Client{Timeout: 15 * time.Minute} 
		req, _ := http.NewRequest("GET", "https://mtgjson.com/api/v5/AllPrintings.json", nil)
		req.Header.Set("User-Agent", config.UserAgent)

		resp, err := bulkClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			callback("Update Failed!", "Bulk download failed.", 0.0, false)
			time.Sleep(2 * time.Second)
			callback("", "", 0.0, true)
			return
		}
		defer resp.Body.Close()

		out, err := os.Create(allPrintingsPath)
		if err != nil {
			callback("Update Failed!", "Disk write error.", 0.0, false)
			time.Sleep(2 * time.Second)
			callback("", "", 0.0, true)
			return
		}
		io.Copy(out, resp.Body)
		out.Close()

		// Save all current sets to tracked_sets.json
		saveTrackedSets(trackedSetsPath, setList.Data)
		
		callback("Update Complete!", "Base DB is ready.", 1.0, false)
		time.Sleep(2 * time.Second)
		callback("", "", 1.0, true)
		return
	}

	// 5. SCENARIO B: The Incremental Sip (Future Updates)
	total := len(missingSets)
	for i, setCode := range missingSets {
		progress := float64(i+1) / float64(total)
		row2 := fmt.Sprintf("Set %d of %d", i+1, total)
		callback(fmt.Sprintf("Fetching %s...", setCode), row2, progress, false)

		req, _ := http.NewRequest("GET", fmt.Sprintf("https://mtgjson.com/api/v5/%s.json", setCode), nil)
		req.Header.Set("User-Agent", config.UserAgent)
		
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == 200 {
			setFilePath := filepath.Join(updatesDir, fmt.Sprintf("%s.json", setCode))
			out, _ := os.Create(setFilePath)
			io.Copy(out, resp.Body)
			out.Close()
		}
		if resp != nil {
			resp.Body.Close()
		}

		// Save state incrementally so if the Pi loses power, we don't redownload
		trackedSets[setCode] = true
		saveTrackedSetsMap(trackedSetsPath, trackedSets)
		
		time.Sleep(100 * time.Millisecond) // Be polite to MTGJSON
	}

	callback("Update Complete!", "New sets added.", 1.0, false)
	time.Sleep(2 * time.Second)
	callback("", "", 1.0, true)
}

// Helpers
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func saveTrackedSets(path string, sets []MTGSetMeta) {
	var codes []string
	for _, s := range sets {
		codes = append(codes, s.Code)
	}
	data, _ := json.MarshalIndent(codes, "", "  ")
	os.WriteFile(path, data, 0644)
}

func saveTrackedSetsMap(path string, tracked map[string]bool) {
	var codes []string
	for code := range tracked {
		codes = append(codes, code)
	}
	data, _ := json.MarshalIndent(codes, "", "  ")
	os.WriteFile(path, data, 0644)
}