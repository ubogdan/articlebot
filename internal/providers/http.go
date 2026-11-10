package providers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

func doJSONRequest(client *http.Client, method, url string, headers map[string]string, payload []byte, maxRetries int) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequest(method, url, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("error creating request: %w", err)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		resp, err := client.Do(req)
		if err != nil {
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("error making request: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("error reading response: %w", readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}

		return respBody, nil
	}
}
