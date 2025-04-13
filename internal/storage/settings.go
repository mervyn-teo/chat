package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Settings struct {
	ApiKey       string `json:"api_key"`
	DiscordToken string `json:"discord_bot_token"`
	Instructions string `json:"instructions"`
}

func LoadSettings(filePath string) (Settings, error) {
	var settings Settings

	jsonFile, err := os.Open(filePath)
	if err != nil {
		return settings, fmt.Errorf("error opening settings file '%s': %w", filePath, err)
	}

	defer func(jsonFile *os.File) {
		err := jsonFile.Close()
		if err != nil {

		}
	}(jsonFile)

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return settings, fmt.Errorf("error reading settings file: %w", err)
	}

	err = json.Unmarshal(byteValue, &settings)
	if err != nil {
		return settings, fmt.Errorf("error decoding settings JSON: %w", err)
	}

	if settings.ApiKey == "" {
		return settings, fmt.Errorf("api_key not found or empty in settings file (expected OpenRouter key)")
	}

	return settings, nil
}
