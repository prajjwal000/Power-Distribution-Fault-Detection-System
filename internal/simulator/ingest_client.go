package simulator

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type IngestClient struct {
	apiURL string
	client *http.Client
}

func NewIngestClient(apiURL string) *IngestClient {
	return &IngestClient{
		apiURL: apiURL,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *IngestClient) Emit(event TelemetryEvent) {
	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("[simulator] marshal error: %v", err)
		return
	}
	req, err := http.NewRequest(http.MethodPost, c.apiURL+"/ingest", bytes.NewReader(body))
	if err != nil {
		log.Printf("[simulator] create request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("[simulator] ingest POST failed (API /ingest not yet implemented): %v", err)
		return
	}
	defer resp.Body.Close() //nolint:errcheck // response body drain in defer, nothing to recover
	if resp.StatusCode >= 400 {
		log.Printf("[simulator] ingest returned status %d", resp.StatusCode)
	}
}
