package tools

import (
	"fmt"
	"log"
	"time"
)

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
