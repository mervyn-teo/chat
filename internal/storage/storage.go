package storage

import (
	"io"
	"log"
	"os"
)

// CheckFileExistence checks if the chat history file exists
func CheckFileExistence(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("INFO: File '%s' does not exist.", filePath)
		return false
	} else if err != nil {
		log.Printf("ERROR: Error checking file existence for '%s': %v", filePath, err)
		return false // Treat error during stat as file not accessible or non-existent for safety
	} else {
		log.Printf("INFO: File '%s' exists.", filePath)
		return true
	}
}

func CreateFile(filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("ERROR: Error creating file '%s': %v", filePath, err)
		return err
	}
	log.Printf("INFO: File '%s' created successfully.", filePath)
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("ERROR: Error closing file '%s' after creation: %v", filePath, err)
			// Note: The original error from CreateFile (if any) is already returned.
			// This deferred close error is logged but not returned to avoid overwriting the original error.
		}
	}()
	return nil
}

func WriteToFile(filePath string, data []byte) error {
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		log.Printf("ERROR: Error opening file '%s' for writing: %v", filePath, err)
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("ERROR: Error closing file '%s' after writing: %v", filePath, err)
		}
	}()

	_, err = file.Write(data)
	if err != nil {
		log.Printf("ERROR: Error writing data to file '%s': %v", filePath, err)
		return err
	}
	log.Printf("INFO: Data written to file '%s' successfully.", filePath)
	return nil
}

func ReadFromFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("ERROR: Error opening file '%s' for reading: %v", filePath, err)
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("ERROR: Error closing file '%s' after reading: %v", filePath, err)
		}
	}()

	byteVal, err := io.ReadAll(file)
	if err != nil {
		log.Printf("ERROR: Error reading data from file '%s': %v", filePath, err)
		return nil, err
	}
	log.Printf("INFO: Data read from file '%s' successfully.", filePath)
	return byteVal, nil
}
