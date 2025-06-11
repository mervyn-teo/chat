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
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"io"
	"log"
	"os"
	"untitled/internal/storage"
)

func LoadConfig() aws.Config {
	log.Println("Loading aws Config")

	err := godotenv.Load(".env")
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

	log.Println(os.Getenv("aws_access_key_id"), os.Getenv("aws_secret_access_key"), os.Getenv("region")) //TODO: remove this line in production

	return awsConfig
}

func TextToSpeech(text string, awsConfig aws.Config) (filename string, err error) {
	if text == "" {
		log.Println("Text input is empty. Please provide valid text.")
		return "", errors.New("text input cannot be empty")
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
		return "", err
	}

	newUUID, err := uuid.NewUUID()

	if err != nil {
		log.Println("Error generating UUID:", err)
	}

	audioFilename := newUUID.String() + ".mp3"

	if storage.CheckFileExistence(audioFilename) {
		err := os.Remove(audioFilename)
		if err != nil {
			return "", err
		}
		log.Println("Removed existing output.mp3 file")
	}

	file, err := os.Create(audioFilename)
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
		return "", err
	}

	log.Println("Speech successfully synthesized and saved to" + audioFilename)
	return audioFilename, nil
}
