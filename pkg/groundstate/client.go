package groundstate

import (
	"bytes"
	"fmt"
	"net/http"
	"time"
)

// PostReport sends a JSON scan report to the Groundstate server.
// url should be the base URL (e.g. "https://app.groundstate.io"); /api/reports is appended automatically.
// token may be empty for unauthenticated servers.
func PostReport(url, token string, body []byte) error {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodPost, url+"/api/reports", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}
