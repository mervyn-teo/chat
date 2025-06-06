package tts

import (
	"testing"
)

func TestTextToSpeech(t *testing.T) {
	t.Run("Valid text input", func(t *testing.T) {
		awsConfig := LoadConfig()
		err := TextToSpeech("Hello from project-local AWS config!", awsConfig)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	t.Run("Empty text input", func(t *testing.T) {
		awsConfig := LoadConfig()
		err := TextToSpeech("", awsConfig)
		if err == nil {
			t.Fatal("Expected error for empty text input, got nil")
		}
	})
}
