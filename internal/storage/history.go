package storage

import (
	"encoding/json"
	"github.com/sashabaranov/go-openai"
	"io"
	"log"
	"os"
)

// CheckFileExistence checks if the chat history file exists, and creates it if it doesn't.
func CheckFileExistence(filePath string) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("Chat history file '%s' does not exist. Creating it... \n", filePath)
	} else {
		log.Printf("Chat history file '%s' already exists. \n", filePath)
		return
	}

	file, err := os.Create(filePath)

	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}

	messages := make(map[string][]openai.ChatCompletionMessage)

	byteVal, err := json.MarshalIndent(messages, "", "  ")

	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}

	_, err = file.Write(byteVal)

	if err != nil {
		log.Fatalf("Error writing to file: %v", err)
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatalf("Error closing file: %v", err)
		}
	}(file)
}

func ReadChatHistory(filePath string) map[string][]openai.ChatCompletionMessage {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatalf("Error closing file: %v", err)
		}
	}(file)

	byteVal, err := io.ReadAll(file)

	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	var messages map[string][]openai.ChatCompletionMessage
	err = json.Unmarshal(byteVal, &messages)

	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}

	return messages
}

func SaveChatHistory(messages map[string][]openai.ChatCompletionMessage, filepath string) {

	file, err := os.Create(filepath)
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatalf("Error closing file: %v", err)
		}
	}(file)

	byteVal, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}

	_, err = file.Write(byteVal)
	if err != nil {
		log.Fatalf("Error writing to file: %v", err)
	}

}
