package mtgdb

import (
	"encoding/json"
	"os"
)

const ParserOptionsPath = "./config/parser_options.json"

// ParserOptions toggles which MTGJSON content is eligible for parsing and
// dithering. Defaults preserve the previously hard-coded filter set.
type ParserOptions struct {
	IncludeUniversesBeyond bool `json:"include_universes_beyond"`
	IncludeSecretLair      bool `json:"include_secret_lair"`
	IncludeFunny           bool `json:"include_funny"`
}

func DefaultParserOptions() ParserOptions {
	return ParserOptions{
		IncludeUniversesBeyond: false,
		IncludeSecretLair:      false,
		IncludeFunny:           false,
	}
}

// LoadParserOptions overlays parser_options.json on top of the defaults.
// A missing file is not an error so the caller can edit-and-retry freely.
func LoadParserOptions(path string) (ParserOptions, error) {
	opts := DefaultParserOptions()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return opts, nil
		}
		return opts, err
	}
	if err := json.Unmarshal(data, &opts); err != nil {
		return opts, err
	}
	return opts, nil
}

// MTGJSON has no set-level marker for Secret Lair releases, so these codes
// are maintained by hand. Add new codes as releases ship.
var secretLairSetCodes = map[string]bool{
	"SLD": true, // Secret Lair Drop
	"SLP": true, // Secret Lair Promo
	"SLU": true, // Secret Lair: Ultimate Edition
	"SLC": true, // Secret Lair Countdown
	"SLX": true, // Universes Within (Secret Lair release)
}

// Universes Beyond set codes are maintained by hand. UB cards that appear
// inside non-UB sets (e.g. Walking Dead Secret Lair drops) will not be
// filtered by this list. Add new codes as releases ship.
var universesBeyondSetCodes = map[string]bool{
	"40K": true, // Warhammer 40,000 Commander
	"LTR": true, // The Lord of the Rings: Tales of Middle-earth
	"LTC": true, // Tales of Middle-earth Commander
	"WHO": true, // Doctor Who
	"PIP": true, // Fallout
	"ACR": true, // Assassin's Creed
	"REX": true, // Jurassic World Collection
	"FIN": true, // Final Fantasy
	"FIC": true, // Final Fantasy Commander
	"TLA": true, // Avatar: The Last Airbender
	"TLC": true, // Avatar: The Last Airbender Commander
	"MAR": true, // Marvel's Spider-Man
	"MAC": true, // Marvel's Spider-Man Commander
}

func (o ParserOptions) skipSet(set MTGSet) bool {
	if set.IsPartialPreview {
		return true
	}
	if !o.IncludeFunny && set.Type == "funny" {
		return true
	}
	if !o.IncludeSecretLair && secretLairSetCodes[set.Code] {
		return true
	}
	if !o.IncludeUniversesBeyond && universesBeyondSetCodes[set.Code] {
		return true
	}
	return false
}
