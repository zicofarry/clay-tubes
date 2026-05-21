package geo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPClient is the real geo-service HTTP client.
// Implements the Client interface by calling clay-geo-service REST endpoints.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPClient creates a real HTTP client for clay-geo-service.
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// RegisterDriver calls POST /internal/drivers/{driverId}/register on geo-service.
func (c *HTTPClient) RegisterDriver(ctx context.Context, u LocationUpdate) error {
	return c.post(ctx, fmt.Sprintf("/internal/drivers/%s/register", u.DriverID), u)
}

// UnregisterDriver calls DELETE /internal/drivers/{driverId} on geo-service.
func (c *HTTPClient) UnregisterDriver(ctx context.Context, driverID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		c.baseURL+"/internal/drivers/"+driverID, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("geo-service returned %d for unregister driver", resp.StatusCode)
	}
	return nil
}

// UpdateLocation calls PUT /internal/drivers/{driverId}/location on geo-service.
func (c *HTTPClient) UpdateLocation(ctx context.Context, u LocationUpdate) error {
	return c.post(ctx, fmt.Sprintf("/internal/drivers/%s/location", u.DriverID), u)
}

func (c *HTTPClient) post(ctx context.Context, path string, body interface{}) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("geo-service call failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("geo-service returned %d for %s", resp.StatusCode, path)
	}
	return nil
}
