package tools

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

// Test structures for JSON unmarshaling
type TimeResponse struct {
	CurrentTime string `json:"current_time"`
}

type DateResponse struct {
	CurrentDate string `json:"current_date"`
}

func TestGetCurrentTime(t *testing.T) {
	t.Run("returns valid time format", func(t *testing.T) {
		result, err := getCurrentTime()

		// Check no error occurred
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Check result is not empty
		if result == "" {
			t.Fatal("Expected non-empty result")
		}

		// Parse JSON response
		var timeResp TimeResponse
		if err := json.Unmarshal([]byte(result), &timeResp); err != nil {
			t.Fatalf("Failed to parse JSON response: %v", err)
		}

		// Validate time format (HH:MM:SS)
		_, err = time.Parse("15:04:05", timeResp.CurrentTime)
		if err != nil {
			t.Fatalf("Invalid time format: %v", err)
		}
	})

	t.Run("logs expected message", func(t *testing.T) {
		// Capture log output
		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer log.SetOutput(os.Stderr) // Reset to default

		_, err := getCurrentTime()
		if err != nil {
			return
		}

		logOutput := buf.String()
		if !strings.Contains(logOutput, "Getting current time...") {
			t.Errorf("Expected log message not found. Got: %s", logOutput)
		}
	})

	t.Run("returns valid JSON structure", func(t *testing.T) {
		result, _ := getCurrentTime()

		// Verify JSON structure
		var jsonMap map[string]interface{}
		if err := json.Unmarshal([]byte(result), &jsonMap); err != nil {
			t.Fatalf("Invalid JSON: %v", err)
		}

		// Check if current_time key exists
		if _, exists := jsonMap["current_time"]; !exists {
			t.Error("JSON response missing 'current_time' key")
		}

		// Check if value is string
		if _, ok := jsonMap["current_time"].(string); !ok {
			t.Error("'current_time' value is not a string")
		}
	})
}

func TestGetCurrentDate(t *testing.T) {
	t.Run("returns valid date format", func(t *testing.T) {
		result, err := getCurrentDate()

		// Check no error occurred
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Check result is not empty
		if result == "" {
			t.Fatal("Expected non-empty result")
		}

		// Parse JSON response
		var dateResp DateResponse
		if err := json.Unmarshal([]byte(result), &dateResp); err != nil {
			t.Fatalf("Failed to parse JSON response: %v", err)
		}

		// Validate date format (YYYY-MM-DD)
		_, err = time.Parse("2006-01-02", dateResp.CurrentDate)
		if err != nil {
			t.Fatalf("Invalid date format: %v", err)
		}
	})

	t.Run("returns today's date", func(t *testing.T) {
		expectedDate := time.Now().Format("2006-01-02")
		result, err := getCurrentDate()

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		var dateResp DateResponse
		json.Unmarshal([]byte(result), &dateResp)

		if dateResp.CurrentDate != expectedDate {
			t.Errorf(
				"Expected date %s, got %s",
				expectedDate, dateResp.CurrentDate,
			)
		}
	})

	t.Run("logs expected message", func(t *testing.T) {
		// Capture log output
		var buf bytes.Buffer
		log.SetOutput(&buf)
		defer log.SetOutput(os.Stderr) // Reset to default

		_, err := getCurrentDate()
		if err != nil {
			return
		}

		logOutput := buf.String()
		if !strings.Contains(logOutput, "Getting current date...") {
			t.Errorf("Expected log message not found. Got: %s", logOutput)
		}
	})

	t.Run("returns valid JSON structure", func(t *testing.T) {
		result, _ := getCurrentDate()

		// Verify JSON structure
		var jsonMap map[string]interface{}
		if err := json.Unmarshal([]byte(result), &jsonMap); err != nil {
			t.Fatalf("Invalid JSON: %v", err)
		}

		// Check if current_date key exists
		if _, exists := jsonMap["current_date"]; !exists {
			t.Error("JSON response missing 'current_date' key")
		}

		// Check if value is string
		if _, ok := jsonMap["current_date"].(string); !ok {
			t.Error("'current_date' value is not a string")
		}
	})
}

// Table-driven test for multiple scenarios
func TestTimeAndDateConsistency(t *testing.T) {
	tests := []struct {
		name        string
		timeFunc    func() (string, error)
		dateFunc    func() (string, error)
		description string
	}{
		{
			name:        "time_and_date_consistency",
			timeFunc:    getCurrentTime,
			dateFunc:    getCurrentDate,
			description: "Time and date should be from the same moment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get both time and date quickly
			timeResult, timeErr := tt.timeFunc()
			dateResult, dateErr := tt.dateFunc()

			// Both should succeed
			if timeErr != nil || dateErr != nil {
				t.Fatalf(
					"Functions should not error: time=%v, date=%v",
					timeErr, dateErr,
				)
			}

			// Parse results
			var timeResp TimeResponse
			var dateResp DateResponse

			json.Unmarshal([]byte(timeResult), &timeResp)
			json.Unmarshal([]byte(dateResult), &dateResp)

			// Verify they represent the same day
			expectedDate := time.Now().Format("2006-01-02")
			if dateResp.CurrentDate != expectedDate {
				t.Errorf(
					"Date inconsistency: expected %s, got %s",
					expectedDate, dateResp.CurrentDate,
				)
			}
		})
	}
}
