package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// sendWebhook sends a webhook to the environment endpoint
func sendWebhook(endpoint string, payload Payload) {
	// Convert payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Error marshaling payload: %v\n", err)
		return // Don't exit on webhook error
	}

	// Create request
	req, err := http.NewRequest("POST", endpoint+"/environments", bytes.NewBuffer(jsonPayload))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send request with retry logic
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Try up to 3 times with exponential backoff
	for i := 0; i < 3; i++ {
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error sending webhook (attempt %d/3): %v\n", i+1, err)
			if i < 2 {
				// Wait before retrying (exponential backoff)
				backoff := time.Duration(1<<uint(i)) * time.Second
				debugLog("Retrying in %v...\n", backoff)
				time.Sleep(backoff)
				continue
			}
			return // Don't exit on webhook error
		}
		defer func() { _ = resp.Body.Close() }()

		// Check response status
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			fmt.Printf("Received non-success status code: %d (attempt %d/3)\n", resp.StatusCode, i+1)
			if i < 2 {
				// Wait before retrying
				backoff := time.Duration(1<<uint(i)) * time.Second
				debugLog("Retrying in %v...\n", backoff)
				time.Sleep(backoff)
				continue
			}
			return // Don't exit on webhook error
		}

		// Success
		debugLog("Drift tracking webhook sent successfully to %s/environments, status: %s\n", endpoint, resp.Status)
		return
	}
}
