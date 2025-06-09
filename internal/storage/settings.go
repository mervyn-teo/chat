package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

type Settings struct {
	ApiKey              string `json:"api_key"`
	DiscordToken        string `json:"discord_bot_token"`
	Instructions        string `json:"instructions"`
	Model               string `json:"model"`
	NewsAPIToken        string `json:"news_api_key"`
	YoutubeToken        string `json:"youtube_api_key"`
	ChatHistoryFilePath string `json:"chat_history_file_path"`
	YoutubeCookies      string `json:"youtube_cookies"`
}

var Setting Settings

func LoadSettings(filePath string) (Settings, error) {

	if !CheckFileExistence(filePath) {
		setUpSettings(filePath)
	}

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
		return Setting, fmt.Errorf("API key is missing in settings file")
	}
	if Setting.DiscordToken == "" {
		return Setting, fmt.Errorf("Discord token is missing in settings file")
	}
	if Setting.NewsAPIToken == "" {
		return Setting, fmt.Errorf("News API key is missing in settings file")
	}
	if Setting.YoutubeToken == "" {
		return Setting, fmt.Errorf("Youtube API key is missing in settings file")
	}

	return Setting, nil
}

func setUpSettings(filepath string) {
	var openRouterApi string
	var discodToken string
	var newsApi string
	var youtubeApi string

	fmt.Printf("Settings file '%s' does not exist. Creating a new one \n", filepath)
	fmt.Println("Please enter your OpenRouter API key: ")
	n, err := fmt.Scanf("%s", &openRouterApi)

	if err != nil {
		log.Fatalln("Error reading OpenRouter API key:", err)
		return
	}

	if n != 1 {
		fmt.Println("Invalid input. Please enter a valid OpenRouter API key.")
		return
	}

	fmt.Println("Please enter your Discord bot token: ")
	n, err = fmt.Scanf("%s", &discodToken)

	if err != nil {
		log.Fatalln("Error reading Discord bot token:", err)
		return
	}

	if n != 1 {
		fmt.Println("Invalid input. Please enter a valid Discord bot token.")
		return
	}

	fmt.Println("Please enter your News API key: ")
	n, err = fmt.Scanf("%s", &newsApi)
	if err != nil {
		return
	}

	if n != 1 {
		fmt.Println("Invalid input. Please enter a valid News API key.")
		return
	}

	fmt.Println("Please enter your Youtube API key: ")
	n, err = fmt.Scanf("%s", &youtubeApi)

	if err != nil {
		return
	}

	if n != 1 {
		fmt.Println("Invalid input. Please enter a valid Youtube API key.")
		return
	}

	// read from example settings file
	exampleFile, err := os.ReadFile("settings.example.json")

	if err != nil {
		fmt.Printf("Error reading example settings file: %v\n", err)
		fmt.Println("Using default settings.")
		defaultSettings := Settings{
			ApiKey:       openRouterApi,
			DiscordToken: discodToken,
			Instructions: "You are a helpful assistant.",
			Model:        "gpt-3.5-turbo",
			NewsAPIToken: newsApi,
			YoutubeToken: youtubeApi,
		}

		file, err := json.MarshalIndent(defaultSettings, "", " ")
		if err != nil {
			fmt.Printf("Error creating default settings file: %v\n", err)
			return
		}

		err = os.WriteFile(filepath, file, 0644)
		if err != nil {
			fmt.Printf("Error writing default settings file: %v\n", err)
			return
		}

		fmt.Printf("Default settings file created at '%s'\n", filepath)
		return
	}

	err = json.Unmarshal(exampleFile, &Setting)

	if err != nil {
		log.Fatalln("Error decoding example settings JSON:", err)
		return
	}
	Setting.ApiKey = openRouterApi
	Setting.DiscordToken = discodToken
	Setting.NewsAPIToken = newsApi
	Setting.YoutubeToken = youtubeApi
	file, err := json.MarshalIndent(Setting, "", " ")

	if err != nil {
		log.Fatalln("Error creating settings file:", err)
		return
	}

	err = os.WriteFile(filepath, file, 0644)

	if err != nil {
		log.Fatalln("Error writing settings file:", err)
		return
	}
}
