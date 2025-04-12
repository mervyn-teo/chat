package openai

import (
	"net/http"

	openai "github.com/sashabaranov/go-openai"
)

const (
	OpenRouterBaseURL = "https://openrouter.ai/api/v1"
	OpenRouterModel   = "deepseek/deepseek-r1:free"
	RefererURL        = "http://localhost"
	AppTitle          = "Go OpenRouter CLI"
)

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

func CreateClient(apiKey string) (*openai.Client, error) {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = OpenRouterBaseURL

	customTransport := &headerTransport{
		Transport: http.DefaultTransport,
	}

	httpClient := &http.Client{
		Transport: customTransport,
	}

	config.HTTPClient = httpClient

	return openai.NewClientWithConfig(config), nil
}
