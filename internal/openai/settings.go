package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Settings struct {
	API_KEY string `json:"api_key"`
}

func LoadAPIKey(filePath string) (string, error) {
	var settings Settings

	jsonFile, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening settings file '%s': %w", filePath, err)
	}

	defer func(jsonFile *os.File) {
		err := jsonFile.Close()
		if err != nil {

		}
	}(jsonFile)

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return "", fmt.Errorf("error reading settings file: %w", err)
	}

	err = json.Unmarshal(byteValue, &settings)
	if err != nil {
		return "", fmt.Errorf("error decoding settings JSON: %w", err)
	}

	if settings.API_KEY == "" {
		return "", fmt.Errorf("api_key not found or empty in settings file (expected OpenRouter key)")
	}

	return settings.API_KEY, nil
}
