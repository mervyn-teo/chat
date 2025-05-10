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
	baseURL := "https://newsapi.org/v2/top-headlines"

	// Construct the URL with query parameters
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}

	// Add query parameters
	params := url.Values{}
	params.Add("country", "us")
	u.RawQuery = params.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create new request: %w", err)
	}

	req.Header.Add("X-Api-Key", storage.Setting.NewsAPIToken)

	// Make the GET request
	client := &http.Client{}
	resp, err := client.Do(req)

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
	baseURL := "https://newsapi.org/v2/everything"

	// Construct the URL with query parameters
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}
	// Add query parameters
	params := url.Values{}
	params.Add("language", "en")
	params.Add("to", args.EndDate)

	if args.PageNumber <= 0 {
		args.PageNumber = 1
	}

	params.Add("page", strconv.Itoa(args.PageNumber))
	params.Add("domains", args.Domain)
	params.Add("excludeDomains", args.DomainsNot)
	params.Add("q", args.Keywords)

	if args.PageSize <= 0 {
		args.PageSize = 10
	}

	params.Add("pageSize", strconv.Itoa(args.PageSize))
	u.RawQuery = params.Encode()

	// Make the GET request
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create new request: %w", err)
	}

	req.Header.Add("X-Api-Key", storage.Setting.NewsAPIToken)

	// Make the GET request
	client := &http.Client{}
	resp, err := client.Do(req)

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
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read response body: %w", err)
		}

		log.Println("News response: " + string(body))
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
