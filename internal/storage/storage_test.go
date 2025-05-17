package storage

import (
	"os"
	"testing"
)

func TestCheckFileExistence(t *testing.T) {
	filePath := "testfile.txt"

	// Create a test file
	err := CreateFile(filePath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	defer func() {
		err := os.Remove("testfile.txt")
		if err != nil {
			// should happen
		}
	}() // Clean up the test file after the test

	// Check if the file exists
	exists := CheckFileExistence(filePath)
	if !exists {
		t.Errorf("Expected file to exist, got false")
	}

	// Delete the test file
	err = os.Remove("testfile.txt")

	if err != nil {
		t.Errorf("Failed to delete test file: %v", err)
	}

	// Check if the file exists after deletion
	exists = CheckFileExistence(filePath)
	if exists {
		t.Errorf("Expected file to not exist, got true")
	}
}

func TestWriteToFile(t *testing.T) {
	err := WriteToFile("testfile.txt", []byte("Hello, World!"))
	if err != nil {
		t.Errorf("Failed to write to file: %v", err)
	}
}
