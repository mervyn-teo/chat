package tools

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"untitled/internal/storage"
)

func getNews() (string, error) {
	log.Printf("User requested news\n")
	baseURL := "https://api.currentsapi.services/v1/latest-news"

	// Construct the URL with query parameters
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}

	// Add query parameters
	params := url.Values{}
	params.Add("language", "en")
	params.Add("apiKey", storage.Setting.NewsAPIToken) // Use the API key from settings
	u.RawQuery = params.Encode()                       // Encode and attach parameters

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

	log.Println("News response: " + string(body))
	return "News: " + string(body), nil
}

func searchNews(args SearchNewsArgs) (string, error) {
	log.Printf("User requested search news\n")
	baseURL := "https://api.currentsapi.services/v1/search"

	// Construct the URL with query parameters
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}
	// Add query parameters
	params := url.Values{}
	params.Add("language", "en")
	params.Add("apiKey", storage.Setting.NewsAPIToken) // Use the API key from settings
	params.Add("end_date", args.EndDate)
	params.Add("type", args.NewsType)
	params.Add("country", args.Country)
	params.Add("category", args.Category)
	params.Add("page_number", strconv.Itoa(args.PageNumber))
	params.Add("domain", args.Domain)
	params.Add("domains_not", args.DomainsNot)
	params.Add("keywords", args.Keywords)
	params.Add("page_size", strconv.Itoa(args.PageSize))
	params.Add("limit", strconv.Itoa(args.Limit))
	u.RawQuery = params.Encode()

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

	log.Println("News response: " + string(body))
	return "News: " + string(body), nil
}
