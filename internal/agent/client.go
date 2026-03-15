package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Invoke(ctx context.Context, messages []RequestMessage) (string, error) {
	reqBody := InvokeRequest{Messages: messages}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/agent/invoke", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(body))
	}

	var invokeResp InvokeResponse
	if err := json.NewDecoder(resp.Body).Decode(&invokeResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Find the last AI message
	for i := len(invokeResp.Output.Messages) - 1; i >= 0; i-- {
		if invokeResp.Output.Messages[i].Type == "ai" {
			return invokeResp.Output.Messages[i].Content, nil
		}
	}

	return "", fmt.Errorf("no AI response found in agent output")
}

func (c *Client) Stream(ctx context.Context, messages []RequestMessage) (<-chan StreamEvent, error) {
	reqBody := InvokeRequest{Messages: messages}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	streamClient := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/agent/stream", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("agent returned status %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan StreamEvent, 32)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		var currentEvent string
		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "event: ") {
				currentEvent = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				event := parseStreamEvent(currentEvent, data)
				if event != nil {
					select {
					case ch <- *event:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return ch, nil
}

func parseStreamEvent(eventType, data string) *StreamEvent {
	switch eventType {
	case "content_block_delta":
		var delta struct {
			Delta struct {
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &delta); err == nil {
			return &StreamEvent{
				Type: "content_block_delta",
				Text: delta.Delta.Text,
			}
		}
	case "message_stop":
		var msg struct {
			Message struct {
				Role string `json:"role"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(data), &msg); err == nil {
			return &StreamEvent{
				Type: "message_stop",
				Role: msg.Message.Role,
			}
		}
	}
	return nil
}
