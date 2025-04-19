// tools/functions.go
package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"untitled/internal/storage"

	openai "github.com/sashabaranov/go-openai"
)

type SearchNewsArgs struct {
	EndDate    string `json:"endDate"`
	NewsType   string `json:"newsType"`
	Country    string `json:"country"`
	Category   string `json:"category"`
	PageNumber int    `json:"pageNumber"`
	Domain     string `json:"domain"`
	DomainsNot string `json:"domainsNot"`
	Keywords   string `json:"keywords"`
	PageSize   int    `json:"pageSize"`
	Limit      int    `json:"limit"`
}

// GetAvailableTools returns a slice of openai.Tool definitions
// that your application supports and wants to expose to the model.
func GetAvailableTools() []openai.Tool {
	timeTool := openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "get_current_time",
			Description: "Get the current time in the user's location",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	dateTool := openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "get_current_date",
			Description: "Get the current date in the user's location",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	newsTool := openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "get_news",
			Description: "Get the latest news headlines",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	searchNewsTool := openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "search_news",
			Description: "search for specific news articles",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"endDate": map[string]interface{}{
						"type":        "string",
						"format":      "date-time",
						"description": "The end date for the news search in RFC 3339 format",
					},
					"newsType": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"1", "2", "3"},
						"description": "The type of news to search for. 1 (news), 2 (articles), 3 (discussion content)",
					},
					"country": map[string]interface{}{
						"type":        "string",
						"description": "The country code for the news search, in uppercase (e.g., 'US' for United States)",
					},
					"category": map[string]interface{}{
						"type": "string",
						"enum": []string{
							"regional",
							"technology",
							"lifestyle",
							"business",
							"general",
							"programming",
							"science",
							"entertainment",
							"world",
							"sports",
							"finance",
							"academia",
							"politics",
							"health",
							"opinion",
							"food",
							"game",
							"fashion",
							"academic",
							"crap",
							"travel",
							"culture",
							"economy",
							"environment",
							"art",
							"music",
							"notsure",
							"CS",
							"education",
							"redundant",
							"television",
							"commodity",
							"movie",
							"entrepreneur",
							"review",
							"auto",
							"energy",
							"celebrity",
							"medical",
							"gadgets",
							"design",
							"EE",
							"security",
							"mobile",
							"estate",
							"funny"},
						"description": "The category of news to search for (e.g., 'technology', 'sports')",
					},
					"pageNumber": map[string]interface{}{
						"type":        "integer",
						"description": "The page number for pagination",
					},
					"domain": map[string]interface{}{
						"type":        "string",
						"description": "The domain to search for news articles (e.g., 'example.com')",
					},
					"domainsNot": map[string]interface{}{
						"type":        "string",
						"description": "Domains to exclude from the search (comma-separated)",
					},
					"keywords": map[string]interface{}{
						"type":        "string",
						"description": "Keywords to search for in the news articles",
					},
					"pageSize": map[string]interface{}{
						"type":        "integer",
						"description": "The number of articles to return per page, up to 20",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "The maximum number of articles to return, up to 20",
					},
				},
			},
		},
	}

	return []openai.Tool{timeTool, dateTool, newsTool, searchNewsTool} // Return a slice of all your tools
}

func getCurrentTime() (string, error) {
	log.Println("Getting current time...")
	currentTime := time.Now().Format("15:04:05") // Format time nicely
	// Return the time as a JSON string
	result := fmt.Sprintf(`{"current_time": "%s"}`, currentTime)
	return result, nil // Success
}

func getCurrentDate() (string, error) {
	log.Println("Getting current date...")
	currentDate := time.Now().Format("2006-01-02") // Format date nicely
	// Return the date as a JSON string
	result := fmt.Sprintf(`{"current_date": "%s"}`, currentDate)
	return result, nil // Success

}

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
	defer resp.Body.Close() // Ensure the response body is closed

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
	defer resp.Body.Close() // Ensure the response body is closed

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

func ExecuteToolCall(toolCall openai.ToolCall) (string, error) {
	if toolCall.Type != openai.ToolTypeFunction {
		log.Printf("Received unexpected tool type: %s", toolCall.Type)
		return fmt.Sprintf(`{"error": "Unsupported tool type: %s"}`, toolCall.Type), fmt.Errorf("unsupported tool type: %s", toolCall.Type)
	}

	// Route the tool call to the correct function based on its name
	switch toolCall.Function.Name {
	case "get_current_time":
		log.Printf("Received call for get_current_time. No arguments expected/parsed.")
		result, funcErr := getCurrentTime()
		if funcErr != nil {
			log.Printf("Error executing function '%s': %v", toolCall.Function.Name, funcErr)
			return fmt.Sprintf(`{"error": "Execution of function '%s' failed: %v"}`, toolCall.Function.Name, funcErr), fmt.Errorf("function execution failed: %w", funcErr)
		}
		return result, nil // Return the result string

	case "get_current_date":
		log.Printf("Received call for get_current_Date. No arguments expected/parsed.")
		result, funcErr := getCurrentDate()
		if funcErr != nil {
			log.Printf("Error executing function '%s': %v", toolCall.Function.Name, funcErr)
			return fmt.Sprintf(`{"error": "Execution of function '%s' failed: %v"}`, toolCall.Function.Name, funcErr), fmt.Errorf("function execution failed: %w", funcErr)
		}
		return result, nil // Return the result string

	case "get_news":
		log.Printf("Received call for get_news. No arguments expected/parsed.")
		result, funcErr := getNews()
		if funcErr != nil {
			log.Printf("Error executing function '%s': %v", toolCall.Function.Name, funcErr)
			return fmt.Sprintf(`{"error": "Execution of function '%s' failed: %v"}`, toolCall.Function.Name, funcErr), fmt.Errorf("function execution failed: %w", funcErr)
		}
		return result, nil // Return the result string
	case "search_news":
		log.Printf("Received call for search_news. Parsing arguments.")
		var args SearchNewsArgs
		err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		log.Println(toolCall.Function.Arguments)

		if err != nil {
			log.Printf("Error parsing arguments for function '%s': %v", toolCall.Function.Name, err)
			return fmt.Sprintf(`{"error": "Failed to parse arguments for function '%s': %v"}`, toolCall.Function.Name, err), fmt.Errorf("argument parsing failed: %w", err)
		}

		result, funcErr := searchNews(args)

		if funcErr != nil {
			log.Printf("Error executing function '%s': %v", toolCall.Function.Name, funcErr)
			return fmt.Sprintf(`{"error": "Execution of function '%s' failed: %v"}`, toolCall.Function.Name, funcErr), fmt.Errorf("function execution failed: %w", funcErr)
		}

		return result, nil // Return the result string
	default:
		log.Printf("Received call for unknown function: %s", toolCall.Function.Name)
		return fmt.Sprintf(`{"error": "Unknown function called: %s"}`, toolCall.Function.Name), fmt.Errorf("unknown function: %s", toolCall.Function.Name)
	}
}
