package mtgdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"momirbox/internal/config"

	"github.com/rs/zerolog/log"
)

// CardVersion represents the MTGJSON v5 schema for a card within a set.
type CardVersion struct {
	Name        string   `json:"name"`
	FaceName    string   `json:"faceName"`
	Layout      string   `json:"layout"`
	Types       []string `json:"types"`
	IsToken     bool     `json:"isToken"`
	Identifiers struct {
		ScryfallID string `json:"scryfallId"`
	} `json:"identifiers"`
	IsFunny    bool    `json:"isFunny"`
	ManaValue  float64 `json:"manaValue"`
	Legalities struct {
		Vintage string `json:"vintage"`
	} `json:"legalities"`
}

// LeanCreature holds the minimal fields required for Momir Basic logic.
type LeanCreature struct {
	Name       string
	CMC        int
	ScryfallID string
}

// TokenVersion represents the MTGJSON v5 schema for a token within a set.
type TokenVersion struct {
	Name         string   `json:"name"`
	FaceName     string   `json:"faceName"`
	Layout       string   `json:"layout"`
	Types        []string `json:"types"`
	TypeLine     string   `json:"type"`
	Text         string   `json:"text"`
	Keywords     []string `json:"keywords"`
	Colors       []string `json:"colors"`
	Power        string   `json:"power"`
	Toughness    string   `json:"toughness"`
	Side         string   `json:"side"`
	IsOnlineOnly bool     `json:"isOnlineOnly"`
	IsRebalanced bool     `json:"isRebalanced"`
	Identifiers  struct {
		ScryfallID string `json:"scryfallId"`
	} `json:"identifiers"`
}

// LeanToken holds filtered metadata for token asset organization and printing.
type LeanToken struct {
	Name       string
	Category   string
	ColorPath  string
	PTPath     string
	Filename   string
	ScryfallID string
	IsBackFace bool
}

// MTGSet represents the root object for a specific Magic set in a JSON stream.
type MTGSet struct {
	Cards  []CardVersion  `json:"cards"`
	Tokens []TokenVersion `json:"tokens"`
}

// ParseAllPrintingsCreatures aggregates legal creatures from bulk and update files.
func ParseAllPrintingsCreatures(cancelChan <-chan struct{}, callback SyncCallback) ([]LeanCreature, error) {
	creatureMap := make(map[string]LeanCreature)
	allPrintingsPath := filepath.Join(config.DataDir, "AllPrintings.json")

	if !FileExists(allPrintingsPath) {
		return nil, fmt.Errorf("AllPrintings.json not found; update database first")
	}

	err := streamBulkFile(allPrintingsPath, "Parsing Creatures...", cancelChan, callback, func(set MTGSet) {
		processSet(set, creatureMap)
	})
	if err != nil && !IsRootCancellation(err) {
		log.Warn().Err(err).Msg("failed to parse bulk creatures")
	}

	if err := processUpdates(cancelChan, func(set MTGSet) {
		processSet(set, creatureMap)
	}); err != nil {
		return nil, err
	}

	list := make([]LeanCreature, 0, len(creatureMap))
	for _, c := range creatureMap {
		list = append(list, c)
	}
	return list, nil
}

// ParseAllPrintingsTokens aggregates unique tokens from bulk and update files.
func ParseAllPrintingsTokens(cancelChan <-chan struct{}, callback SyncCallback) ([]LeanToken, error) {
	tokenMap := make(map[string]LeanToken)
	allPrintingsPath := filepath.Join(config.DataDir, "AllPrintings.json")

	if !FileExists(allPrintingsPath) {
		return nil, fmt.Errorf("AllPrintings.json not found; update database first")
	}

	err := streamBulkFile(allPrintingsPath, "Parsing Tokens...", cancelChan, callback, func(set MTGSet) {
		processTokens(set, tokenMap)
	})
	if err != nil && !IsRootCancellation(err) {
		log.Warn().Err(err).Msg("failed to parse bulk tokens")
	}

	if err := processUpdates(cancelChan, func(set MTGSet) {
		processTokens(set, tokenMap)
	}); err != nil {
		return nil, err
	}

	list := make([]LeanToken, 0, len(tokenMap))
	for _, t := range tokenMap {
		list = append(list, t)
	}
	return list, nil
}

func processSet(set MTGSet, creatureMap map[string]LeanCreature) {
	for _, card := range set.Cards {
		if card.Layout == "token" || card.IsToken || contains(card.Types, "Token") {
			continue
		}
		if !contains(card.Types, "Creature") || strings.ToLower(card.Legalities.Vintage) != "legal" {
			continue
		}
		if card.IsFunny || card.Identifiers.ScryfallID == "" {
			continue
		}

		name := card.FaceName
		if name == "" {
			name = strings.Split(card.Name, " // ")[0]
		}

		if _, exists := creatureMap[name]; !exists {
			creatureMap[name] = LeanCreature{
				Name:       name,
				CMC:        int(card.ManaValue),
				ScryfallID: card.Identifiers.ScryfallID,
			}
		}
	}
}

func processTokens(set MTGSet, tokenMap map[string]LeanToken) {
	for _, token := range set.Tokens {
		name := token.FaceName
		if name == "" {
			name = token.Name
		}

		if token.Layout == "art_series" || contains(token.Types, "Card") || token.TypeLine == "Card" {
			continue
		}
		if strings.Contains(name, "Substitute") || strings.Contains(name, "Checklist") {
			continue
		}
		if token.IsOnlineOnly || token.IsRebalanced || token.Identifiers.ScryfallID == "" {
			continue
		}

		verifiedKeywords := verifyKeywords(token.Text, token.Keywords)
		colors := "C"
		if len(token.Colors) > 0 {
			colors = strings.Join(token.Colors, "")
		}

		p, t := token.Power, token.Toughness
		if p == "" {
			p = "?"
		}
		if t == "" {
			t = "?"
		}

		identity := fmt.Sprintf("%s|%s|%s|%s|%v", name, colors, p, t, verifiedKeywords)
		if _, exists := tokenMap[identity]; !exists {
			tokenMap[identity] = buildLeanToken(name, colors, p, t, verifiedKeywords, token)
		}
	}
}

func buildLeanToken(name, colors, p, t string, keywords []string, raw TokenVersion) LeanToken {
	fileName := name
	if len(keywords) > 0 {
		fileName = fmt.Sprintf("%s (%s)", name, strings.Join(keywords, ", "))
	}

	category := "helpers"
	if contains(raw.Types, "Emblem") || strings.Contains(raw.TypeLine, "Emblem") {
		category = "emblems"
		fileName = strings.NewReplacer("Emblem - ", "", "Emblem ", "").Replace(fileName)
	} else if contains(raw.Types, "Creature") {
		category = "creatures"
	} else if contains(raw.Types, "Artifact") {
		category = "artifacts"
	}

	return LeanToken{
		Name:       name,
		Category:   category,
		ColorPath:  colors,
		PTPath:     fmt.Sprintf("%s_%s", p, t),
		Filename:   fileName,
		ScryfallID: raw.Identifiers.ScryfallID,
		IsBackFace: raw.Side == "b",
	}
}

func processUpdates(cancelChan <-chan struct{}, processFn func(MTGSet)) error {
	updatesDir := filepath.Join(config.DataDir, "updates")
	entries, err := os.ReadDir(updatesDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		select {
		case <-cancelChan:
			return context.Canceled
		default:
		}

		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			path := filepath.Join(updatesDir, entry.Name())
			if err := streamUpdateFile(path, cancelChan, processFn); err != nil {
				if IsRootCancellation(err) {
					return err
				}
				log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to parse update file")
			}
		}
	}
	return nil
}

type byteCounter struct {
	io.Reader
	tracker    *ProgressTracker
	callback   SyncCallback
	cancelChan <-chan struct{}
	title      string
	current    int64
	lastUpdate time.Time
}

func (bc *byteCounter) Read(p []byte) (int, error) {
	select {
	case <-bc.cancelChan:
		return 0, context.Canceled
	default:
	}

	n, err := bc.Reader.Read(p)
	bc.current += int64(n)

	if time.Since(bc.lastUpdate) > 200*time.Millisecond {
		progress, etaStr := bc.tracker.GetETA(float64(bc.current))
		bc.callback(bc.title, etaStr, progress, false)
		bc.lastUpdate = time.Now()
	}
	return n, err
}

func streamBulkFile(path string, title string, cancelChan <-chan struct{}, callback SyncCallback, fn func(MTGSet)) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, _ := file.Stat()
	decoder := json.NewDecoder(&byteCounter{
		Reader:     file,
		tracker:    NewTracker(float64(stat.Size())),
		callback:   callback,
		cancelChan: cancelChan,
		title:      title,
	})

	if err := advanceToData(decoder); err != nil {
		return err
	}

	if _, err := decoder.Token(); err != nil {
		return err
	}

	for decoder.More() {
		if _, err := decoder.Token(); err != nil {
			return err
		}
		var set MTGSet
		if err := decoder.Decode(&set); err == nil {
			fn(set)
		}
	}
	return nil
}

func streamUpdateFile(path string, cancelChan <-chan struct{}, fn func(MTGSet)) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := advanceToData(decoder); err != nil {
		return err
	}

	var set MTGSet
	if err := decoder.Decode(&set); err == nil {
		select {
		case <-cancelChan:
			return context.Canceled
		default:
			fn(set)
		}
	}
	return nil
}

func advanceToData(decoder *json.Decoder) error {
	for {
		t, err := decoder.Token()
		if err != nil {
			return err
		}
		if key, ok := t.(string); ok && key == "data" {
			return nil
		}
	}
}

func verifyKeywords(text string, keywords []string) []string {
	lowerText := strings.ToLower(text)
	var verified []string
	for _, kw := range keywords {
		if strings.Contains(lowerText, strings.ToLower(kw)) {
			verified = append(verified, kw)
		}
	}
	return verified
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
