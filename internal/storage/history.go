package storage

import (
	"encoding/json"
	"github.com/sashabaranov/go-openai"
	"log"
)

func CreateChatHistoryFile(filePath string) {
	log.Println("Creating chat history file...")

	err := CreateFile(filePath)

	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}

	messages := make(map[string][]openai.ChatCompletionMessage)
	byteVal, err := json.MarshalIndent(messages, "", "  ")

	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}

	err = WriteToFile(filePath, byteVal)

	if err != nil {
		log.Fatalf("Error writing to file: %v", err)
	}
}

func ReadChatHistory(filePath string) map[string][]openai.ChatCompletionMessage {
	var messages map[string][]openai.ChatCompletionMessage
	byteVal, err := ReadFromFile(filePath)

	if err != nil {
		log.Fatalf("Error reading file: %v", err)
		return nil
	}

	err = json.Unmarshal(byteVal, &messages)

	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}

	return messages
}

func SaveChatHistory(messages map[string][]openai.ChatCompletionMessage, filepath string) {

	err := CreateFile(filepath)

	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}

	byteVal, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}

	err = WriteToFile(filepath, byteVal)

	if err != nil {
		log.Fatalf("Error writing to file: %v", err)
	}
}
