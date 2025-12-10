package httpclient

import (
	"io"
	"net/http/httptest"
	"net/http"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHTTPClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HTTP Client Suite")
}

var _ = Describe("HTTP Client", func() {

	// Declare BEFORE using it
	var testServer *httptest.Server

	BeforeEach(func() {

		// Create mock HTTP test server
		testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case "GET":
				w.Write([]byte(`{"status":"ok"}`))
			case "POST":
				w.Write([]byte(`{"created":true}`))
			case "PUT":
				w.Write([]byte(`{"updated":true}`))
			case "DELETE":
				w.Write([]byte(`{"deleted":true}`))
			}
		}))

		// Set BaseURL to test server URL
		os.Setenv("AI_SERVICES_BASE_URL", testServer.URL)
	})

	AfterEach(func() {
		testServer.Close()
	})

	It("GET works", func() {
		client := NewHTTPClient()
		resp, err := client.Get("/test")
		Expect(err).To(BeNil())

		body, _ := io.ReadAll(resp.Body)
		Expect(string(body)).To(ContainSubstring("ok"))
	})

	It("POST works", func() {
		client := NewHTTPClient()
		resp, err := client.Post("/test", map[string]string{"a": "b"})
		Expect(err).To(BeNil())

		body, _ := io.ReadAll(resp.Body)
		Expect(string(body)).To(ContainSubstring("created"))
	})

	It("PUT works", func() {
		client := NewHTTPClient()
		resp, err := client.Put("/test", map[string]string{"x": "y"})
		Expect(err).To(BeNil())

		body, _ := io.ReadAll(resp.Body)
		Expect(string(body)).To(ContainSubstring("updated"))
	})

	It("DELETE works", func() {
		client := NewHTTPClient()
		resp, err := client.Delete("/test")
		Expect(err).To(BeNil())

		body, _ := io.ReadAll(resp.Body)
		Expect(string(body)).To(ContainSubstring("deleted"))
	})
})