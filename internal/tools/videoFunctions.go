package tools

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"untitled/internal/storage"
)

func getVideo(keyWord string) (string, error) {
	log.Println("Getting video...")
	baseURL := "https://www.googleapis.com/youtube/v3/search"

	// Construct the URL with query parameters
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}

	// Add query parameters
	params := url.Values{}
	params.Add("part", "snippet")
	params.Add("q", keyWord)
	params.Add("maxResults", "5") // Limit to 5 results
	params.Add("type", "video")
	params.Add("key", storage.Setting.YoutubeToken) // Use the API key from settings

	u.RawQuery = params.Encode() // Encode and attach parameters
	// Make the GET request
	resp, err := http.Get(u.String())
	if err != nil {
		return "", fmt.Errorf("failed to make GET request: %w", err)
	}

	defer func() {
		err = resp.Body.Close()
		if err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}() // Ensure the response body is closed
	// Check for successful response status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	log.Println("Video response: " + string(body))
	return "Video: " + string(body), nil
}
