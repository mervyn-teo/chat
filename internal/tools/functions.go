// tools/functions.go
package tools

import (
	"encoding/json"
	"fmt"
	openai "github.com/sashabaranov/go-openai"
	"log"
)

type SearchNewsArgs struct {
	EndDate    string `json:"endDate"`
	PageNumber int    `json:"pageNumber"`
	Domain     string `json:"domain"`
	DomainsNot string `json:"domainsNot"`
	Keywords   string `json:"keywords"`
	PageSize   int    `json:"pageSize"`
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
						"description": "Keywords to search for in the news articles, MUST be in english",
					},
					"pageSize": map[string]interface{}{
						"type":        "integer",
						"description": "The number of articles to return per page, up to 20",
					},
				},
				"required": []string{"keywords"},
			},
		},
	}

	searchVideoTool := openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "search_video",
			Description: "Search for specific video from youtube",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"keywords": map[string]interface{}{
						"type":        "string",
						"description": "Keywords to search for in the video",
					},
				},
			},
		},
	}

	createReminderTool := openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "create_reminder",
			Description: "Create a reminder",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "The title of the reminder",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "The description of the reminder",
					},
					"time": map[string]interface{}{
						"type":        "string",
						"description": "The time of the reminder in 'YYYY-MM-DDTHH:MM:SS' format",
					},
					"user_id": map[string]interface{}{
						"type":        "string",
						"description": "The user ID to send the reminder to",
					},
					"channel_id": map[string]interface{}{
						"type":        "string",
						"description": "The channel ID to send the reminder to",
					},
				},
			},
		},
	}

	getRemindersTool := openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "list_reminders",
			Description: "List all reminders",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	return []openai.Tool{timeTool, dateTool, newsTool, searchNewsTool, searchVideoTool, createReminderTool, getRemindersTool} // Return a slice of all your tools
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

	case "search_video":
		log.Printf("Received call for search_video. Parsing arguments.")
		var args map[string]string
		err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		if err != nil {
			log.Printf("Error parsing arguments for function '%s': %v", toolCall.Function.Name, err)
			return fmt.Sprintf(`{"error": "Failed to parse arguments for function '%s': %v"}`, toolCall.Function.Name, err), fmt.Errorf("argument parsing failed: %w", err)
		}

		result, funcErr := getVideo(args["keywords"])
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
