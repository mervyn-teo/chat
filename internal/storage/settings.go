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
	Model        string `json:"model"`
	NewsAPIToken string `json:"news_api_key"`
	YoutubeToken string `json:"youtube_api_key"`
}

var Setting Settings

func LoadSettings(filePath string) (Settings, error) {

	jsonFile, err := os.Open(filePath)
	if err != nil {
		return Setting, fmt.Errorf("error opening settings file '%s': %w", filePath, err)
	}

	defer func(jsonFile *os.File) {
		err := jsonFile.Close()
		if err != nil {

		}
	}(jsonFile)

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return Setting, fmt.Errorf("error reading settings file: %w", err)
	}

	err = json.Unmarshal(byteValue, &Setting)
	if err != nil {
		return Setting, fmt.Errorf("error decoding settings JSON: %w", err)
	}

	if Setting.ApiKey == "" {
		return Setting, fmt.Errorf("api_key not found or empty in settings file (expected OpenRouter key)")
	}

	return Setting, nil
}
