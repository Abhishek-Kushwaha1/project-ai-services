package httpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type HTTPClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewHTTPClient() *HTTPClient {
	baseURL := os.Getenv("AI_SERVICES_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &HTTPClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *HTTPClient) buildURL(path string) string {
	return fmt.Sprintf("%s%s", c.BaseURL, path)
}

func (c *HTTPClient) Get(path string) (*http.Response, error) {
	return c.HTTPClient.Get(c.buildURL(path))
}

func (c *HTTPClient) Post(path string, body interface{}) (*http.Response, error) {
	b, _ := json.Marshal(body)
	return c.HTTPClient.Post(c.buildURL(path), "application/json", bytes.NewBuffer(b))
}

func (c *HTTPClient) Put(path string, body interface{}) (*http.Response, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", c.buildURL(path), bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	return c.HTTPClient.Do(req)
}

func (c *HTTPClient) Delete(path string) (*http.Response, error) {
	req, _ := http.NewRequest("DELETE", c.buildURL(path), nil)
	return c.HTTPClient.Do(req)
}
