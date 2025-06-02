package tts

import (
	"context"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Run("Non-empty config", func(t *testing.T) { // Load AWS configuration
		awsConfig := LoadConfig(".testAWSConfig")
		creds, err := awsConfig.Credentials.Retrieve(context.TODO())
		if err != nil {
			return
		}

		// Check if the configuration is not nil
		if awsConfig.Credentials == nil || awsConfig.Region == "" {
			t.Fatalf("Expected valid AWS configuration, got %v", awsConfig)
		}

		// Check if credentials are set
		if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
			t.Fatal("Expected valid AWS credentials, but found empty values")
		}

		// Check if access key ID and secret access key are correct
		if creds.AccessKeyID != "12345678901234567890" || creds.SecretAccessKey != "098765432109876543210" {
			t.Fatal("Expected valid AWS credentials, but found incorrect values")
		}

		// Check if region is set
		if awsConfig.Region == "" {
			t.Fatal("Expected valid AWS region, but found empty value")
		}

	})
}

func TestTextToSpeech(t *testing.T) {
	t.Run("Valid text input", func(t *testing.T) {
		awsConfig := LoadConfig("../../.aws-creds")
		err := TextToSpeech("Hello from project-local AWS config!", awsConfig)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	t.Run("Empty text input", func(t *testing.T) {
		awsConfig := LoadConfig(".testAWSConfig")
		err := TextToSpeech("", awsConfig)
		if err == nil {
			t.Fatal("Expected error for empty text input, got nil")
		}
	})
}
