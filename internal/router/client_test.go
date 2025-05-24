package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultModel(t *testing.T) {
	expected := "gpt-3.5-turbo"
	if OpenRouterModel != expected {
		t.Errorf("Expected default model to be %q, got %q",
			expected, OpenRouterModel)
	}
}

func TestHeaderTransport_RoundTrip(t *testing.T) {
	tests := []struct {
		name             string
		transport        http.RoundTripper
		expectedReferer  string
		expectedTitle    string
		shouldUseDefault bool
	}{
		{
			name:             "with custom transport",
			transport:        &mockTransport{},
			expectedReferer:  RefererURL,
			expectedTitle:    AppTitle,
			shouldUseDefault: false,
		},
		{
			name:             "with nil transport (uses default)",
			transport:        nil,
			expectedReferer:  RefererURL,
			expectedTitle:    AppTitle,
			shouldUseDefault: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server to capture the request
			server := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					// Check headers
					referer := r.Header.Get("HTTP-Referer")
					title := r.Header.Get("X-Title")

					if referer != tt.expectedReferer {
						t.Errorf("Expected HTTP-Referer to be %q, got %q",
							tt.expectedReferer, referer)
					}

					if title != tt.expectedTitle {
						t.Errorf("Expected X-Title to be %q, got %q",
							tt.expectedTitle, title)
					}

					w.WriteHeader(http.StatusOK)
				}))
			defer server.Close()

			// Create headerTransport
			headerTransport := &headerTransport{
				Transport: tt.transport,
			}

			// Create request
			req, err := http.NewRequest("GET", server.URL, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Execute request
			resp, err := headerTransport.RoundTrip(req)
			if err != nil {
				t.Fatalf("RoundTrip failed: %v", err)
			}
			defer func(Body io.ReadCloser) {
				err := Body.Close()
				if err != nil {

				}
			}(resp.Body)

			// Verify response
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status code %d, got %d",
					http.StatusOK, resp.StatusCode)
			}
		})
	}
}

func TestCreateClient(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		apiKey        string
		expectError   bool
		expectedModel string
	}{
		{
			name:          "valid parameters",
			model:         "gpt-4",
			apiKey:        "test-api-key",
			expectError:   false,
			expectedModel: "gpt-4",
		},
		{
			name:          "empty model",
			model:         "",
			apiKey:        "test-api-key",
			expectError:   false,
			expectedModel: "",
		},
		{
			name:          "empty api key",
			model:         "gpt-3.5-turbo",
			apiKey:        "",
			expectError:   false,
			expectedModel: "gpt-3.5-turbo",
		},
		{
			name:          "different model",
			model:         "claude-3-opus",
			apiKey:        "test-api-key-2",
			expectError:   false,
			expectedModel: "claude-3-opus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store original model to restore later
			originalModel := OpenRouterModel

			client, err := CreateClient(tt.model, tt.apiKey)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check client creation
			if !tt.expectError {
				if client == nil {
					t.Error("Expected client to be created but got nil")
				}

				// Check if global model was updated
				if OpenRouterModel != tt.expectedModel {
					t.Errorf("Expected OpenRouterModel to be %q, got %q",
						tt.expectedModel, OpenRouterModel)
				}
			}

			// Restore original model
			OpenRouterModel = originalModel
		})
	}
}

func TestCreateClient_Integration(t *testing.T) {
	// Test that the client is properly configured
	model := "gpt-4"
	apiKey := "test-api-key"

	client, err := CreateClient(model, apiKey)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client == nil {
		t.Fatal("Client should not be nil")
	}
}

func TestCreateClient_ModelUpdate(t *testing.T) {
	originalModel := OpenRouterModel
	defer func() {
		OpenRouterModel = originalModel
	}()

	testModel := "test-model"
	_, err := CreateClient(testModel, "test-key")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if OpenRouterModel != testModel {
		t.Errorf("Expected OpenRouterModel to be updated to %q, got %q",
			testModel, OpenRouterModel)
	}
}

func TestHeaderTransport_HeadersAreSet(t *testing.T) {
	// Create a mock server that captures headers
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			capturedHeaders = r.Header.Clone()
			w.WriteHeader(http.StatusOK)
		}))
	defer server.Close()

	// Create client with custom transport
	transport := &headerTransport{Transport: http.DefaultTransport}
	client := &http.Client{Transport: transport}

	// Make request
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	// Verify headers were set
	if capturedHeaders.Get("HTTP-Referer") != RefererURL {
		t.Errorf("Expected HTTP-Referer to be %q, got %q",
			RefererURL, capturedHeaders.Get("HTTP-Referer"))
	}

	if capturedHeaders.Get("X-Title") != AppTitle {
		t.Errorf("Expected X-Title to be %q, got %q",
			AppTitle, capturedHeaders.Get("X-Title"))
	}
}

// Benchmark tests
func BenchmarkCreateClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := CreateClient("gpt-3.5-turbo", "test-api-key")
		if err != nil {
			b.Fatalf("Failed to create client: %v", err)
		}
	}
}

func BenchmarkHeaderTransportRoundTrip(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	defer server.Close()

	transport := &headerTransport{Transport: http.DefaultTransport}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, err := transport.RoundTrip(req)
		if err != nil {
			b.Fatalf("RoundTrip failed: %v", err)
		}
		err = resp.Body.Close()
		if err != nil {
			return
		}
	}
}

// Mock transport for testing
type mockTransport struct{}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create a simple response
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       http.NoBody,
		Request:    req,
	}
	return resp, nil
}

// Test helper functions
func TestMockTransport(t *testing.T) {
	transport := &mockTransport{}
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d",
			http.StatusOK, resp.StatusCode)
	}
}

// Edge case tests
func TestHeaderTransport_PreservesExistingHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Check that both original and new headers exist
			if r.Header.Get("Custom-Header") != "custom-value" {
				t.Errorf("Original header was not preserved")
			}
			if r.Header.Get("HTTP-Referer") != RefererURL {
				t.Errorf("New header was not added")
			}
			w.WriteHeader(http.StatusOK)
		}))
	defer server.Close()

	transport := &headerTransport{Transport: http.DefaultTransport}

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Add a custom header
	req.Header.Set("Custom-Header", "custom-value")

	_, err = transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}
}
