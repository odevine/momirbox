package mtgdb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"momirbox/internal/config"
)

// CardVersion matches the structure inside a Set in MTGJSON v5.
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

// LeanCreature represents the minimal data needed for the Momir list.
type LeanCreature struct {
	Name       string
	CMC        int
	ScryfallID string
}

// TokenVersion matches the token structure inside a Set.
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

// LeanToken represents the minimal data needed for the tokens list.
type LeanToken struct {
	Name       string
	Category   string
	ColorPath  string
	PTPath     string
	Filename   string
	ScryfallID string
	IsBackFace bool
}

// MTGSet represents the root object for a specific Magic set.
type MTGSet struct {
	Cards  []CardVersion  `json:"cards"`
	Tokens []TokenVersion `json:"tokens"`
}

// ParseAllPrintingsCreatures loads all Magic: The Gathering creature data from the local database.
// It processes both the main AllPrintings.json file and any update files, returning a deduplicated list of creatures.
func ParseAllPrintingsCreatures() ([]LeanCreature, error) {
	creatureMap := make(map[string]LeanCreature)

	allPrintingsPath := filepath.Join(config.DataDir, "AllPrintings.json")
	updatesDir := filepath.Join(config.DataDir, "updates")

	if _, err := os.Stat(allPrintingsPath); err == nil {
		err := streamBulkFile(allPrintingsPath, func(set MTGSet) {
			processSet(set, creatureMap)
		})
		if err != nil {
			fmt.Printf("Warning: Failed to parse AllPrintings.json creatures: %v\n", err)
		}
	} else {
		return nil, fmt.Errorf("AllPrintings.json not found. Please Update DB first")
	}

	if entries, err := os.ReadDir(updatesDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				setPath := filepath.Join(updatesDir, entry.Name())
				err := streamUpdateFile(setPath, func(set MTGSet) {
					processSet(set, creatureMap)
				})
				if err != nil {
					fmt.Printf("Warning: Failed to parse update file %s creatures: %v\n", entry.Name(), err)
				}
			}
		}
	}

	var momirList []LeanCreature
	for _, c := range creatureMap {
		momirList = append(momirList, c)
	}

	return momirList, nil
}

// ParseAllPrintingsTokens builds the complete token list from the bulk file AND the updates folder.
func ParseAllPrintingsTokens() ([]LeanToken, error) {
	// We use a string key for deduplication: "Name|Colors|Power|Toughness|Keywords"
	tokenMap := make(map[string]LeanToken)

	allPrintingsPath := filepath.Join(config.DataDir, "AllPrintings.json")
	updatesDir := filepath.Join(config.DataDir, "updates")

	if _, err := os.Stat(allPrintingsPath); err == nil {
		err := streamBulkFile(allPrintingsPath, func(set MTGSet) {
			processTokens(set, tokenMap)
		})
		if err != nil {
			fmt.Printf("Warning: Failed to parse AllPrintings.json tokens: %v\n", err)
		}
	} else {
		return nil, fmt.Errorf("AllPrintings.json not found. Please Update DB first")
	}

	if entries, err := os.ReadDir(updatesDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				setPath := filepath.Join(updatesDir, entry.Name())
				err := streamUpdateFile(setPath, func(set MTGSet) {
					processTokens(set, tokenMap)
				})
				if err != nil {
					fmt.Printf("Warning: Failed to parse update file %s tokens: %v\n", entry.Name(), err)
				}
			}
		}
	}

	var momirList []LeanToken
	for _, t := range tokenMap {
		momirList = append(momirList, t)
	}

	return momirList, nil
}

// processSet applies the Momir Basic legality rules to a single set and adds valid cards to the map.
func processSet(set MTGSet, creatureMap map[string]LeanCreature) {
	for _, card := range set.Cards {
		if card.Layout == "token" || card.IsToken || contains(card.Types, "Token") {
			continue
		}
		if !contains(card.Types, "Creature") {
			continue
		}
		if strings.ToLower(card.Legalities.Vintage) != "legal" {
			continue
		}
		if card.IsFunny || card.Identifiers.ScryfallID == "" {
			continue
		}

		// Handle double-faced cards safely
		frontName := card.FaceName
		if frontName == "" {
			frontName = strings.Split(card.Name, " // ")[0]
		}

		if _, exists := creatureMap[frontName]; !exists {
			creatureMap[frontName] = LeanCreature{
				Name:       frontName,
				CMC:        int(card.ManaValue),
				ScryfallID: card.Identifiers.ScryfallID,
			}
		}
	}
}

// processTokens adds tokens from a set to the token map, applying various filters and deduplication.
func processTokens(set MTGSet, tokenMap map[string]LeanToken) {
	for _, token := range set.Tokens {
		name := token.FaceName
		if name == "" {
			name = token.Name
		}

		// Junk filters
		if token.Layout == "art_series" || contains(token.Types, "Card") || token.TypeLine == "Card" {
			continue
		}
		if strings.Contains(name, "Substitute") || strings.Contains(name, "Checklist") {
			continue
		}
		if token.IsOnlineOnly || token.IsRebalanced || token.Identifiers.ScryfallID == "" {
			continue
		}

		// Verify keywords actually appear in text
		faceTextLower := strings.ToLower(token.Text)
		var verifiedKeywords []string
		for _, kw := range token.Keywords {
			if strings.Contains(faceTextLower, strings.ToLower(kw)) {
				verifiedKeywords = append(verifiedKeywords, kw)
			}
		}

		// Sort colors for consistent pathing
		colors := "C"
		if len(token.Colors) > 0 {
			colors = strings.Join(token.Colors, "") 
		}

		power := token.Power
		if power == "" {
			power = "?"
		}
		toughness := token.Toughness
		if toughness == "" {
			toughness = "?"
		}

		// Unique identity key
		identity := fmt.Sprintf("%s|%s|%s|%s|%v", name, colors, power, toughness, verifiedKeywords)

		if _, exists := tokenMap[identity]; !exists {
			// Build filename base: "Name (Keyword1, Keyword2)"
			fileNameBase := name
			if len(verifiedKeywords) > 0 {
				fileNameBase = fmt.Sprintf("%s (%s)", name, strings.Join(verifiedKeywords, ", "))
			}

			// Determine Category
			category := "helpers"
			if contains(token.Types, "Emblem") || strings.Contains(token.TypeLine, "Emblem") {
				category = "emblems"
				fileNameBase = strings.ReplaceAll(fileNameBase, "Emblem - ", "")
				fileNameBase = strings.ReplaceAll(fileNameBase, "Emblem ", "")
			} else if contains(token.Types, "Creature") {
				category = "creatures"
			} else if contains(token.Types, "Artifact") {
				category = "artifacts"
			}

			tokenMap[identity] = LeanToken{
				Name:       name,
				Category:   category,
				ColorPath:  colors,
				PTPath:     fmt.Sprintf("%s_%s", power, toughness),
				Filename:   fileNameBase,
				ScryfallID: token.Identifiers.ScryfallID,
				IsBackFace: token.Side == "b",
			}
		}
	}
}

// streamBulkFile opens AllPrintings.json and runs a custom function on every Set it finds.
func streamBulkFile(filePath string, processFn func(MTGSet)) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := advanceToData(decoder); err != nil {
		return err
	}

	decoder.Token() // Consume '{'
	for decoder.More() {
		decoder.Token() // Read set code
		var set MTGSet
		if err := decoder.Decode(&set); err == nil {
			processFn(set) 
		}
	}
	return nil
}

// streamUpdateFile opens a single [SET].json and runs a custom function on it.
func streamUpdateFile(filePath string, processFn func(MTGSet)) error {
	file, err := os.Open(filePath)
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
		processFn(set) 
	}
	return nil
}

// advanceToData is a helper to advance the JSON stream to the "data" root key.
func advanceToData(decoder *json.Decoder) error {
	for {
		t, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("failed to find 'data' key: %w", err)
		}
		if key, ok := t.(string); ok && key == "data" {
			return nil
		}
	}
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}