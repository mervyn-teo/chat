package router

import (
	"log"
	"net/http"

	openai "github.com/sashabaranov/go-openai"
)

const (
	OpenRouterBaseURL = "https://openrouter.ai/api/v1"
	RefererURL        = "http://localhost"
	AppTitle          = "Go OpenRouter CLI"
)

var OpenRouterModel = "gpt-3.5-turbo" // default model

type headerTransport struct {
	Transport http.RoundTripper
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Transport
	if base == nil {
		base = http.DefaultTransport
	}

	req.Header.Set("HTTP-Referer", RefererURL)
	req.Header.Set("X-Title", AppTitle)

	return base.RoundTrip(req)
}

func CreateClient(model string, apiKey string) (*openai.Client, error) {
	OpenRouterModel = model
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = OpenRouterBaseURL

	log.Println("You are using " + OpenRouterModel + " model, you can change it in settings.json")
	customTransport := &headerTransport{
		Transport: http.DefaultTransport,
	}

	httpClient := &http.Client{
		Transport: customTransport,
	}

	config.HTTPClient = httpClient

	return openai.NewClientWithConfig(config), nil
}
