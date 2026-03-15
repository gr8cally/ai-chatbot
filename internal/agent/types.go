package agent

type RequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type InvokeRequest struct {
	Messages []RequestMessage `json:"messages"`
}

type InvokeResponse struct {
	Output struct {
		Messages []ResponseMessage `json:"messages"`
	} `json:"output"`
}

type ResponseMessage struct {
	Content string `json:"content"`
	Type    string `json:"type"` // "human" or "ai"
	ID      string `json:"id"`
}

type StreamEvent struct {
	Type string // "content_block_delta", "message_stop", "error"
	Text string
	Role string // "human" or "assistant" (for message_stop)
}
