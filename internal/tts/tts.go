package tts

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	"github.com/aws/aws-sdk-go-v2/service/polly/types"
	_ "github.com/aws/aws-sdk-go-v2/service/polly/types"
	"github.com/joho/godotenv"
	"io"
	"log"
	"os"
	"untitled/internal/storage"
)

func LoadConfig() aws.Config {

	err := godotenv.Load("../../.env")
	if err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
	}

	awsConfig := aws.Config{
		Credentials: credentials.NewStaticCredentialsProvider(
			os.Getenv("aws_access_key_id"),
			os.Getenv("aws_secret_access_key"),
			"",
		),
		Region: os.Getenv("region"),
	}

	return awsConfig
}

func TextToSpeech(text string, awsConfig aws.Config) error {
	if text == "" {
		log.Println("Text input is empty. Please provide valid text.")
		return errors.New("text input cannot be empty")
	}

	client := polly.NewFromConfig(awsConfig)
	output, err := client.SynthesizeSpeech(context.TODO(), &polly.SynthesizeSpeechInput{
		Text:         aws.String(text),
		OutputFormat: types.OutputFormatMp3,
		VoiceId:      types.VoiceIdJoanna,
		Engine:       types.EngineStandard,
	})

	if err != nil {
		log.Println("Error synthesizing speech:", err)
		return err
	}

	if storage.CheckFileExistence("output.mp3") {
		err := os.Remove("output.mp3")
		if err != nil {
			return err
		}
		log.Println("Removed existing output.mp3 file")
	}

	file, err := os.Create("output.mp3")
	if err != nil {
		log.Fatal("Error creating output file:", err)
	}
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			log.Println("Error closing output file:", err)
		}
	}(file)

	_, err = io.Copy(file, output.AudioStream)
	if err != nil {
		log.Printf("Error writing audio stream to file: %v", err)
		return err
	}

	log.Println("Speech successfully synthesized and saved to output.mp3")
	return nil
}
