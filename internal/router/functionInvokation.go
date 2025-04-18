package router

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"untitled/internal/storage"
)

type function struct {
	name string
	args []string
}

// parseModelResponse parses the user input and checks for function invocations
// Returns the cleaned response and a boolean indicating if a function was invoked
// If a function is invoked, it extracts the function name and arguments
// If no function is invoked, it returns the original response
func parseModelResponse(modelResponse string) (string, bool) {
	ret := modelResponse
	isFunc := false

	if strings.Contains(modelResponse, "!func") {
		ret = extractFunction(strings.Split(modelResponse, "!func")[1])
		isFunc = true
	}

	return ret, isFunc
}

func extractFunction(text string) string {
	log.Println("Bot requested: " + text)
	// clean up the text
	text = strings.ReplaceAll(text, " ", "")
	text = strings.ReplaceAll(text, "\n", "")

	splitFunctions := strings.Split(text, ",")
	var functions []function

	for i := 0; i < len(splitFunctions); i++ {
		functionName := strings.Split(splitFunctions[i], "(")[0]
		extractedArgs := strings.Split(splitFunctions[i], "(")[1]
		extractedArgs = strings.ReplaceAll(extractedArgs, ")", "")
		splitArgs := strings.Split(extractedArgs, ",")

		functions = append(functions, function{})
		functions[i].name = functionName
		functions[i].args = make([]string, len(splitArgs))
	}

	reply := ""

	for i := 0; i < len(functions); i++ {
		reply += "{" + invokeFunction(functions[i]) + "}, "
	}

	log.Println(reply)
	return reply
}

func invokeFunction(f function) string {
	res := ""

	switch f.name {
	case "currTime":
		res = currTime()
	case "currDate":
		res = currDate()
	case "getNews":
		news, err := getNews()
		if err != nil {
			res = fmt.Sprintf("Error fetching news: %v", err)
		}
		res = news
	default:
		res = "Function not found"
	}

	return res
}

func currTime() string {
	currtime := time.Now()
	rettime := currtime.Format("15:04:05")
	log.Printf("User requested current time: %s", rettime)
	return rettime
}

func currDate() string {
	// Implement the logic to get the current time
	currdate := time.Now()
	retdate := currdate.Format("02-01-2006")
	log.Printf("User requested current date: %s", retdate)
	return retdate
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
	return string(body), nil
}
