package storage

import (
	"io"
	"log"
	"os"
)

// CheckFileExistence checks if the chat history file exists
func CheckFileExistence(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("File '%s' does not exist. \n", filePath)
		return false
	} else {
		log.Printf("File '%s' exists. \n", filePath)
		return true
	}
}

func CreateFile(filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
		return err
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatalf("Error closing file: %v", err)
		}
	}(file)

	return nil
}

func WriteToFile(filePath string, data []byte) error {
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)

	if err != nil {
		log.Fatalf("Error opening file: %v", err)
		return err
	}

	_, err = file.Write(data)

	if err != nil {
		log.Fatalf("Error writing to file: %v", err)
		return err
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatalf("Error closing file: %v", err)
		}
	}(file)

	return nil
}

func ReadFromFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
		return nil, err
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
		return nil, err
	}

	return byteVal, nil
}
