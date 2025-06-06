package tts

import (
	"bufio"
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	"github.com/aws/aws-sdk-go-v2/service/polly/types"
	_ "github.com/aws/aws-sdk-go-v2/service/polly/types"
	"io"
	"log"
	"os"
	"strings"
	"untitled/internal/storage"
)

func LoadConfig(filepath string) aws.Config {
	file, err := os.Open(filepath)
	if err != nil {
		log.Fatal(err)
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	// Assuming the file contains AWS credentials in a format that can be parsed
	var accessKey, secretKey, region string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "aws_access_key_id") {
			accessKey = strings.Split(line, "=")[1]
		} else if strings.HasPrefix(line, "aws_secret_access_key") {
			secretKey = strings.Split(line, "=")[1]
		} else if strings.HasPrefix(line, "region") {
			region = strings.Split(line, "=")[1]
		}
	}

	awsConfig := aws.Config{
		Credentials: credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(accessKey),
			strings.TrimSpace(secretKey),
			"",
		),
		Region: strings.TrimSpace(region),
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
