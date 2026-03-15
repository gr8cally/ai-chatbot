# AI Chatbot Interface — Implementation Plan

This document provides a phased implementation plan for the AI chatbot interface described in `spec.md`. Each phase has clear aims, deliverables, and success conditions so that any developer (human or LLM) can pick up and execute independently.

---

## Prerequisites

Before starting, ensure the following tools are installed:

| Tool | Version | Install |
|---|---|---|
| Go | >= 1.22 | https://go.dev/dl/ |
| Templ | >= 0.2.x | `go install github.com/a-h/templ/cmd/templ@latest` |
| Node.js | >= 20 | For TypeScript compilation (esbuild/tsc) |
| Air (optional) | latest | `go install github.com/air-verse/air@latest` — hot-reload for dev |

Verify the mock API is reachable:

```bash
curl -s -X POST https://ai4d.wiremockapi.cloud/agent/invoke \
  -H "Content-Type: application/json" \
  -d '{"messages": [{"role": "user", "content": "Hello"}]}' | head -c 200
```

You should see a JSON response with `output.messages`.

---

## Project Structure (Target)

```
ai-chatbot/
├── .env-example
├── .env                     # local only, gitignored
├── .gitignore
├── go.mod
├── go.sum
├── spec.md
├── implementation.md
├── main.go                  # entry point: load config, start server
├── internal/
│   ├── config/
│   │   └── config.go        # env var loading
│   ├── session/
│   │   └── store.go         # in-memory session store
│   ├── agent/
│   │   ├── client.go        # HTTP client for agent API
│   │   └── types.go         # request/response types
│   └── handler/
│       ├── chat.go           # HTTP handlers (send, stream, abort, clear)
│       └── page.go           # page rendering handlers
├── templates/
│   ├── layout.templ          # base HTML layout
│   ├── chat.templ            # chat page
│   ├── message.templ         # message bubble component
│   ├── stream.templ          # streaming placeholder component
│   └── error.templ           # error message component
├── static/
│   ├── css/
│   │   └── styles.css
│   └── js/
│       ├── vendor/
│       │   ├── htmx.min.js
│       │   └── htmx-sse.js
│       └── dist/             # compiled TS output
│           └── app.js
├── ts/
│   ├── main.ts               # entry point, event listeners
│   ├── streaming.ts           # SSE client + animation
│   ├── markdown.ts            # markdown rendering
│   ├── clipboard.ts           # copy-to-clipboard
│   └── autoscroll.ts          # auto-scroll logic
├── tsconfig.json
└── Makefile                   # build commands
```

---

## Phase 1: Project Scaffolding & Static Chat UI

### Aim
Set up the Go project, Templ templates, and static CSS to render a hardcoded chat page. No API calls yet — just prove the build pipeline and layout work.

### Steps

1. **Initialize Go module**
   ```bash
   go mod init github.com/<your-org>/ai-chatbot
   ```

2. **Install Go dependencies**
   ```bash
   go get github.com/a-h/templ
   go get github.com/joho/godotenv
   go get github.com/google/uuid
   ```

3. **Create `.env-example` and `.gitignore`**
   - `.env-example`:
     ```env
     # Required: Base URL for the LangChain Agent API
     AGENT_URL=https://ai4d.wiremockapi.cloud

     # Optional: Server port (default: 8080)
     PORT=8080

     # Optional: Session cookie signing secret
     SESSION_SECRET=change-me-in-production
     ```
   - `.gitignore`: include `.env`, `*_templ.go`, `static/js/dist/`, `tmp/`, `node_modules/`

4. **Create `internal/config/config.go`**
   - Load `AGENT_URL` (required, panic if missing), `PORT` (default `8080`), `SESSION_SECRET` (default random)
   - Use `godotenv.Load()` to read `.env`

5. **Create Templ templates**
   - `templates/layout.templ`: full HTML page shell with `<head>` (CSS, JS links) and `<body>` with a slot for children
   - `templates/chat.templ`: renders the chat page with:
     - A header bar
     - A scrollable message container (`id="chat-messages"`)
     - An input form at the bottom (`id="chat-form"`)
   - `templates/message.templ`: a single message bubble component accepting `Role`, `Content`, `Timestamp`
     - Conditionally apply CSS classes based on role (user vs assistant)

6. **Create `static/css/styles.css`**
   - Full-viewport-height layout (flexbox)
   - Header, scrollable messages area, fixed input bar
   - Message bubble styles (user right-aligned blue, assistant left-aligned gray)
   - Responsive: max-width container centered, works at 320px+

7. **Create `main.go`**
   - Load config
   - Set up `http.ServeMux` with:
     - `GET /` → render chat page with 2-3 hardcoded messages
     - `GET /static/` → file server for static assets
   - Start HTTP server

8. **Create `Makefile`**
   ```makefile
   .PHONY: generate build run dev

   generate:
       templ generate

   build: generate
       go build -o bin/chatbot .

   run: build
       ./bin/chatbot

   dev:
       air
   ```

9. **Run `templ generate` and `go run .`** — verify the page renders at `http://localhost:8080`

### Success Conditions
- [ ] `make run` starts the server without errors
- [ ] Navigating to `http://localhost:8080` shows a styled chat page
- [ ] Hardcoded user and assistant message bubbles are visible with correct alignment
- [ ] The layout is responsive (test at 375px and 1440px widths)
- [ ] The input form is visible and has a text field + send button

---

## Phase 2: Session Management & Non-Streaming Chat

### Aim
Implement the in-memory session store, wire up the input form to send messages to the agent `/agent/invoke` endpoint, and display the response. This delivers a **fully functional (non-streaming) chatbot**.

### Steps

1. **Create `internal/session/store.go`**
   - `SessionStore` with `sync.RWMutex` and `map[string]*Session`
   - Methods:
     - `GetOrCreate(sessionID string) *Session` — return existing or create new
     - `AddMessage(sessionID string, msg Message)` — append to session
     - `GetMessages(sessionID string) []Message` — return copy of messages
     - `Clear(sessionID string)` — reset messages
   - Generate session IDs with `github.com/google/uuid`

2. **Create `internal/agent/types.go`**
   - Define request/response structs matching the API contract from `spec.md` section 3.2:
     ```go
     type InvokeRequest struct {
         Messages []RequestMessage `json:"messages"`
     }
     type RequestMessage struct {
         Role    string `json:"role"`    // "user" or "assistant"
         Content string `json:"content"`
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
     ```

3. **Create `internal/agent/client.go`**
   - `AgentClient` struct with `baseURL string` and `*http.Client` (with timeout)
   - `Invoke(ctx context.Context, messages []RequestMessage) (string, error)`:
     - POST to `{baseURL}/agent/invoke` with JSON body
     - Parse response, find last message with `type == "ai"`, return its `content`
     - Handle HTTP errors (non-2xx), JSON parse errors, network errors
     - Return descriptive error messages for each failure mode

4. **Create `internal/handler/chat.go`**
   - **`HandleSend(w, r)`** (`POST /chat/send`):
     1. Read session ID from cookie (or create new session + set cookie)
     2. Extract `message` from form values
     3. Validate: reject empty messages
     4. Add user message to session store
     5. Build `[]RequestMessage` from session history
     6. Call `agentClient.Invoke(ctx, messages)`
     7. On success: add assistant message to session store, render both new bubbles as HTML fragment
     8. On error: render error bubble HTML fragment
   - **`HandleClear(w, r)`** (`POST /chat/clear`):
     1. Clear session messages
     2. Return empty chat container HTML fragment (with welcome message)

5. **Update `templates/chat.templ`**
   - Remove hardcoded messages
   - Add HTMX attributes to the form:
     ```html
     <form hx-post="/chat/send"
           hx-target="#chat-messages"
           hx-swap="beforeend"
           hx-disabled-elt="find input, find button"
           hx-indicator="#loading-indicator">
       <input type="text" name="message" placeholder="Type a message..."
              autocomplete="off" required>
       <button type="submit">Send</button>
     </form>
     ```
   - Add loading indicator element (`id="loading-indicator"`)
   - Add clear button with `hx-post="/chat/clear"` `hx-target="#chat-messages"` `hx-swap="innerHTML"`

6. **Create `templates/error.templ`**
   - Error bubble component: red-tinted, left-aligned, shows error message
   - Includes a "Retry" button (re-sends the last user message)

7. **Update `main.go`**
   - Initialize `SessionStore` and `AgentClient`
   - Register `/chat/send` and `/chat/clear` handlers

8. **Test end-to-end:**
   ```bash
   # Start server
   make run

   # Test invoke
   curl -X POST http://localhost:8080/chat/send \
     -H "Content-Type: application/x-www-form-urlencoded" \
     -d "message=What+is+attention"
   ```

### Success Conditions
- [ ] Typing a message and pressing Send calls the agent API and displays the response
- [ ] The input field and send button are disabled while waiting for a response
- [ ] A loading indicator (dots or spinner) appears during API call
- [ ] Chat history persists across messages within the same session (conversation context)
- [ ] Refreshing the page preserves chat history (session cookie)
- [ ] Clicking "Clear" resets the conversation
- [ ] Network errors show a user-friendly error message in the chat
- [ ] Empty message submission is prevented

---

## Phase 3: Streaming via SSE

### Aim
Replace the non-streaming invoke flow with real-time streaming. The Go server proxies the agent's SSE stream and re-emits events to the browser. This is the **core differentiating feature**.

### Steps

1. **Add streaming method to `internal/agent/client.go`**
   - `Stream(ctx context.Context, messages []RequestMessage) (<-chan StreamEvent, error)`:
     - POST to `{baseURL}/agent/stream` with JSON body
     - Read the response body line-by-line using `bufio.Scanner`
     - Parse SSE format: lines starting with `event:` and `data:`
     - Emit parsed events to a channel
     - Close channel when stream ends or context is cancelled
   - Define `StreamEvent` types:
     ```go
     type StreamEvent struct {
         Type string // "content_block_delta", "message_stop", "error"
         Text string // for delta events: the text chunk
         Raw  string // raw JSON data
     }
     ```

2. **Create stream handlers in `internal/handler/chat.go`**
   - **`HandleStreamStart(w, r)`** (`POST /chat/stream/start`):
     1. Read session, extract message, add user message to session
     2. Generate a unique stream ID (UUID)
     3. Store the pending stream request in a `sync.Map` (streamID → context + messages)
     4. Return HTML fragment:
        - User message bubble
        - Assistant placeholder bubble with SSE connection:
          ```html
          <div id="assistant-stream"
               hx-ext="sse"
               sse-connect="/chat/stream?stream_id=XXX"
               sse-swap="delta"
               hx-swap="beforeend">
            <span class="cursor blink"></span>
          </div>
          ```
   - **`HandleStream(w, r)`** (`GET /chat/stream`):
     1. Set response headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`
     2. Retrieve pending stream request by stream ID
     3. Call `agentClient.Stream(ctx, messages)`
     4. For each `content_block_delta` event, write SSE to the response:
        ```
        event: delta
        data: <html-escaped text chunk>

        ```
     5. On `message_stop` (for assistant role): write final SSE event `event: done`, then close
     6. Flush after each event using `http.Flusher`
     7. On error: write `event: error` with error message
     8. After stream completes: add full assistant message to session store

   - **`HandleAbort(w, r)`** (`POST /chat/abort`):
     1. Look up the active stream's `context.CancelFunc` by session ID
     2. Call cancel
     3. Return 200 OK

3. **Update `templates/chat.templ`**
   - Change form to use streaming endpoint:
     ```html
     <form hx-post="/chat/stream/start"
           hx-target="#chat-messages"
           hx-swap="beforeend"
           ...>
     ```
   - Add abort button (hidden by default, shown during streaming):
     ```html
     <button id="abort-btn"
             hx-post="/chat/abort"
             class="hidden">
       Stop generating
     </button>
     ```

4. **Create `templates/stream.templ`**
   - Streaming assistant bubble template with SSE attributes and cursor animation

5. **Test streaming end-to-end:**
   - Open browser, send a message
   - Verify text appears incrementally
   - Verify abort button cancels generation

### Success Conditions
- [ ] Sending a message opens an SSE connection and text streams in real-time
- [ ] Each text chunk from the agent appears as it arrives (no waiting for full response)
- [ ] The input is disabled during streaming
- [ ] The abort button appears during streaming and disappears after completion
- [ ] Clicking abort stops the stream and shows partial response
- [ ] After streaming completes, the full response is saved to session history
- [ ] Subsequent messages include previous conversation context
- [ ] Network errors during streaming show an error message in the chat
- [ ] The SSE connection closes cleanly after the response is complete

---

## Phase 4: TypeScript — Streaming Animation & Markdown Rendering

### Aim
Add progressive enhancement with TypeScript: animate streaming text, render markdown with syntax-highlighted code blocks, and add copy-to-clipboard on code blocks.

### Steps

1. **Set up TypeScript build**
   - Create `tsconfig.json` (target ES2020, module ESNext)
   - Install esbuild: `npm init -y && npm install --save-dev esbuild`
   - Add to Makefile:
     ```makefile
     ts:
         npx esbuild ts/main.ts --bundle --outfile=static/js/dist/app.js --format=iife --minify
     ```

2. **Install frontend libraries**
   ```bash
   npm install marked highlight.js dompurify
   npm install --save-dev @types/dompurify
   ```

3. **Create `ts/markdown.ts`**
   ```typescript
   import { marked } from 'marked';
   import hljs from 'highlight.js';
   import DOMPurify from 'dompurify';

   // Configure marked to use highlight.js for code blocks
   marked.setOptions({
     highlight: (code, lang) => {
       if (lang && hljs.getLanguage(lang)) {
         return hljs.highlight(code, { language: lang }).value;
       }
       return hljs.highlightAuto(code).value;
     },
   });

   export function renderMarkdown(raw: string): string {
     const html = marked.parse(raw) as string;
     return DOMPurify.sanitize(html);
   }

   // Incremental rendering: debounced, used during streaming
   let debounceTimer: number | null = null;
   export function renderMarkdownIncremental(element: HTMLElement, raw: string): void {
     if (debounceTimer) clearTimeout(debounceTimer);
     debounceTimer = window.setTimeout(() => {
       element.innerHTML = renderMarkdown(raw);
       addCopyButtons(element);
     }, 80);
   }
   ```

4. **Create `ts/streaming.ts`**
   - Instead of relying purely on HTMX SSE swap (which inserts raw text), use a custom JS `EventSource` listener:
     1. Listen for HTMX `htmx:sseOpen` or manually create `EventSource`
     2. On each `delta` event: append text to an accumulator string
     3. Call `renderMarkdownIncremental(targetEl, accumulated)` to re-render
     4. Animate: optionally add a brief CSS transition on the content or use a typewriter effect
     5. On `done` event: final `renderMarkdown`, close EventSource, re-enable input
     6. On `error` event: show error, re-enable input

   - **Animation approach**: Rather than character-by-character (which is slow for large chunks), use a **fade-in per chunk** strategy:
     - Each incoming chunk is wrapped in a `<span class="chunk-fade">` that starts at `opacity: 0` and transitions to `opacity: 1` over 150ms
     - This creates a smooth "appearing" effect without blocking rendering

5. **Create `ts/clipboard.ts`**
   ```typescript
   export function addCopyButtons(container: HTMLElement): void {
     container.querySelectorAll('pre code').forEach((block) => {
       const pre = block.parentElement!;
       if (pre.querySelector('.copy-btn')) return; // already added

       const btn = document.createElement('button');
       btn.className = 'copy-btn';
       btn.textContent = 'Copy';
       btn.addEventListener('click', async () => {
         await navigator.clipboard.writeText(block.textContent || '');
         btn.textContent = 'Copied!';
         setTimeout(() => { btn.textContent = 'Copy'; }, 2000);
       });
       pre.style.position = 'relative';
       pre.appendChild(btn);
     });
   }
   ```

6. **Create `ts/autoscroll.ts`**
   ```typescript
   export function setupAutoScroll(containerId: string): void {
     const container = document.getElementById(containerId)!;
     let userScrolledUp = false;

     container.addEventListener('scroll', () => {
       const atBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 50;
       userScrolledUp = !atBottom;
     });

     const observer = new MutationObserver(() => {
       if (!userScrolledUp) {
         container.scrollTop = container.scrollHeight;
       }
     });

     observer.observe(container, { childList: true, subtree: true, characterData: true });
   }
   ```

7. **Create `ts/main.ts`**
   - Import and initialize all modules
   - Set up event listeners for HTMX events (`htmx:afterSwap`, etc.)
   - Initialize auto-scroll on page load

8. **Add CSS for code blocks and animations**
   - highlight.js theme CSS (e.g., `github-dark` theme)
   - `.copy-btn` positioning (absolute, top-right of `<pre>`)
   - `.chunk-fade` animation keyframes
   - Blinking cursor animation for streaming placeholder

9. **Update Makefile**: add `ts` target to the `build` and `dev` targets

10. **Update `templates/layout.templ`**: reference `/static/js/dist/app.js` and highlight.js CSS

### Success Conditions
- [ ] Agent responses render as proper markdown (headings, bold, italic, lists, links)
- [ ] Code blocks have syntax highlighting with correct colors
- [ ] Each code block has a "Copy" button that copies the code to clipboard
- [ ] Clicking "Copy" shows "Copied!" feedback for 2 seconds
- [ ] During streaming, markdown re-renders incrementally (text doesn't jump or flash excessively)
- [ ] New chunks have a subtle fade-in animation
- [ ] The chat auto-scrolls as new content arrives during streaming
- [ ] If the user scrolls up manually, auto-scroll pauses
- [ ] Scrolling back to bottom resumes auto-scroll
- [ ] A blinking cursor appears in the assistant bubble during streaming

---

## Phase 5: Error Handling, Edge Cases & UX Polish

### Aim
Harden the application against real-world failure modes and polish the UX details.

### Steps

1. **Error handling — network failures**
   - Agent client: configurable timeout (default 30s for invoke, 60s for stream)
   - Retry logic: no automatic retries (let the user decide), but show a "Retry" button on error
   - On SSE disconnect mid-stream: show error message with partial content preserved

2. **Error handling — malformed responses**
   - If agent returns non-JSON or unexpected structure: show generic "Something went wrong" error
   - If SSE events have unexpected format: log warning, skip event, continue

3. **Input validation**
   - Trim whitespace, reject empty
   - Optional: max message length (e.g., 4000 chars) with character counter
   - Prevent double-submit (HTMX `hx-disabled-elt` handles this, but verify)

4. **Empty state**
   - When no messages exist, show a centered welcome screen:
     - App title/logo
     - Brief description
     - 2-3 suggested prompt buttons that pre-fill the input
   - Suggested prompts use `hx-on:click` to fill the input and auto-submit

5. **Abort generation polish**
   - Show abort button only during streaming (toggle visibility via HTMX events or JS)
   - After abort: show the partial response with a note "(generation stopped)"
   - Re-enable input immediately after abort

6. **Keyboard shortcuts**
   - `Enter` to send (already works with form)
   - `Shift+Enter` for newline (convert `<input>` to `<textarea>` with auto-resize)
   - `Escape` to abort generation

7. **Timestamps**
   - Show relative timestamps on messages ("just now", "2 min ago")
   - Update timestamps periodically (every 60s)

8. **Accessibility**
   - `role="log"` and `aria-live="polite"` on message container
   - `aria-label` on buttons
   - Focus management: after sending, focus returns to input
   - Skip-to-content link

9. **Mobile responsiveness final pass**
   - Test on 375px (iPhone SE), 390px (iPhone 14), 768px (tablet)
   - Ensure input doesn't get hidden behind virtual keyboard
   - Touch-friendly button sizes (min 44x44px)

### Success Conditions
- [ ] Network timeout shows a clear error with "Retry" option
- [ ] Malformed API responses don't crash the app
- [ ] Empty messages cannot be sent
- [ ] Fresh session shows a welcome screen with suggested prompts
- [ ] Clicking a suggested prompt sends it as a message
- [ ] Abort button only appears during streaming, disappears after
- [ ] Partial responses are preserved after abort
- [ ] `Shift+Enter` creates a newline in the input
- [ ] `Escape` aborts generation
- [ ] The app is usable with keyboard-only navigation
- [ ] The app works well on mobile viewport widths

---

## Phase 6: Build, Documentation & Delivery

### Aim
Finalize the project for delivery: production build, README, and clean repo.

### Steps

1. **Production build**
   - Makefile `build` target produces a single binary + static assets
   - Verify the binary runs standalone:
     ```bash
     AGENT_URL=https://ai4d.wiremockapi.cloud ./bin/chatbot
     ```

2. **Download and vendor frontend dependencies**
   - htmx: download `htmx.min.js` and `sse.js` extension to `static/js/vendor/`
   - highlight.js CSS theme to `static/css/vendor/`
   - Ensure `npm run build` (or esbuild) bundles TS + dependencies into `static/js/dist/app.js`

3. **Create `README.md`**
   - Project overview
   - Screenshots/GIF of the chatbot in action
   - Prerequisites
   - Quick start:
     ```bash
     cp .env-example .env
     # edit .env with your AGENT_URL
     make build
     make run
     # open http://localhost:8080
     ```
   - Development workflow (`make dev` with air)
   - Architecture overview (link to spec.md)
   - Environment variables reference

4. **Clean up**
   - Remove any debug logging
   - Ensure `.gitignore` covers: `.env`, `bin/`, `tmp/`, `node_modules/`, `*_templ.go`
   - Verify no secrets or credentials in codebase

5. **Final verification checklist**
   - [ ] `git clone` + `make build` + `make run` works from scratch
   - [ ] All features from spec.md are implemented
   - [ ] Responsive on mobile and desktop
   - [ ] No console errors in browser
   - [ ] Streaming works end-to-end
   - [ ] Error states are handled gracefully

### Success Conditions
- [ ] A fresh `git clone` → `make build` → `make run` works with only Go and Node.js installed
- [ ] The README is clear enough for a new developer to get started in under 5 minutes
- [ ] `.env-example` documents all environment variables
- [ ] No secrets, debug logs, or TODO comments remain in the codebase
- [ ] The app passes all success conditions from Phases 1-5

---

## Appendix A: Key Implementation Patterns

### A.1 SSE Proxy Pattern (Go)

The Go server acts as an SSE proxy between the agent API and the browser. This is the most complex piece of the implementation.

```go
func (h *Handler) HandleStream(w http.ResponseWriter, r *http.Request) {
    // 1. Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming not supported", http.StatusInternalServerError)
        return
    }

    // 2. Get messages from session
    streamID := r.URL.Query().Get("stream_id")
    pendingReq := h.pendingStreams.Load(streamID)
    messages := pendingReq.Messages

    // 3. Create cancellable context
    ctx, cancel := context.WithCancel(r.Context())
    defer cancel()
    h.activeStreams.Store(sessionID, cancel)
    defer h.activeStreams.Delete(sessionID)

    // 4. Call agent stream
    events, err := h.agent.Stream(ctx, messages)
    if err != nil {
        fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
        flusher.Flush()
        return
    }

    // 5. Re-emit events
    var fullText strings.Builder
    for event := range events {
        switch event.Type {
        case "content_block_delta":
            fullText.WriteString(event.Text)
            escaped := html.EscapeString(event.Text)
            fmt.Fprintf(w, "event: delta\ndata: %s\n\n", escaped)
            flusher.Flush()
        case "message_stop":
            fmt.Fprintf(w, "event: done\ndata: complete\n\n")
            flusher.Flush()
        case "error":
            fmt.Fprintf(w, "event: error\ndata: %s\n\n", event.Text)
            flusher.Flush()
        }
    }

    // 6. Save complete message to session
    h.sessions.AddMessage(sessionID, Message{
        Role:    RoleAssistant,
        Content: fullText.String(),
    })
}
```

### A.2 SSE Parsing Pattern (Go)

```go
func (c *AgentClient) Stream(ctx context.Context, messages []RequestMessage) (<-chan StreamEvent, error) {
    body, _ := json.Marshal(InvokeRequest{Messages: messages})
    req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/agent/stream", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("stream request failed: %w", err)
    }

    ch := make(chan StreamEvent, 16)
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
```

### A.3 HTMX + SSE Integration Pattern (Browser)

Rather than using HTMX's built-in SSE swap (which is limited to HTML insertion), use a **hybrid approach**:

1. HTMX handles the initial form submission and swaps in the placeholder bubble
2. TypeScript takes over for SSE event handling (using `EventSource` API) to enable markdown rendering and animation
3. HTMX events (`htmx:afterSwap`) trigger TS initialization

```typescript
// After HTMX swaps in the streaming placeholder
document.addEventListener('htmx:afterSwap', (event) => {
  const target = document.getElementById('assistant-stream');
  if (!target) return;

  const sseUrl = target.dataset.sseUrl;
  if (!sseUrl) return;

  const es = new EventSource(sseUrl);
  let accumulated = '';

  es.addEventListener('delta', (e) => {
    accumulated += e.data;
    renderMarkdownIncremental(target, accumulated);
  });

  es.addEventListener('done', () => {
    es.close();
    target.innerHTML = renderMarkdown(accumulated);
    addCopyButtons(target);
    enableInput();
  });

  es.addEventListener('error', (e) => {
    es.close();
    showError(target, 'Stream interrupted');
    enableInput();
  });
});
```

### A.4 Templ Component Pattern

```go
// templates/message.templ

templ MessageBubble(msg Message) {
    <div class={ "message", templ.KV("message-user", msg.Role == "user"),
                             templ.KV("message-assistant", msg.Role == "assistant") }>
        <div class="message-content">
            if msg.Role == "assistant" {
                // Content will be rendered as markdown by TS
                <div class="markdown-body" data-raw={ msg.Content }>
                    // Server-side: render raw text as fallback
                    // JS will replace with rendered markdown on load
                    { msg.Content }
                </div>
            } else {
                <p>{ msg.Content }</p>
            }
        </div>
        <div class="message-meta">
            <span class="message-time">{ formatTime(msg.Timestamp) }</span>
        </div>
    </div>
}
```

---

## Appendix B: Testing Checklist

Use this checklist for manual testing after each phase:

### Functional
- [ ] Send a message → receive a response
- [ ] Send multiple messages → conversation context is maintained
- [ ] Clear conversation → chat is empty, new messages don't include old context
- [ ] Refresh page → chat history is preserved (same session)
- [ ] Open in incognito → new empty session
- [ ] Streaming: text appears token-by-token
- [ ] Abort: stops generation, shows partial response
- [ ] Markdown: headings, bold, italic, lists, links render correctly
- [ ] Code blocks: syntax highlighted, copy button works
- [ ] Error: disconnect wifi → error message shown

### UX
- [ ] Loading indicator visible during generation
- [ ] Input disabled during generation
- [ ] Auto-scroll follows new content
- [ ] Manual scroll-up pauses auto-scroll
- [ ] Scroll back to bottom resumes auto-scroll
- [ ] Enter sends message, Shift+Enter adds newline
- [ ] Escape aborts generation
- [ ] Mobile: layout works, keyboard doesn't overlap input

### Edge Cases
- [ ] Very long message (1000+ words) renders without breaking layout
- [ ] Very long code block scrolls horizontally
- [ ] Rapid successive messages don't cause race conditions
- [ ] Agent returns LaTeX notation → renders as text (not broken)
- [ ] Empty agent response → handled gracefully
- [ ] Session cookie deleted → new session created transparently

---

## Appendix C: Useful Commands

```bash
# Generate templ files
templ generate

# Build TypeScript
npx esbuild ts/main.ts --bundle --outfile=static/js/dist/app.js --format=iife

# Build everything
make build

# Run with hot-reload (requires air)
make dev

# Test agent invoke endpoint
curl -s -X POST https://ai4d.wiremockapi.cloud/agent/invoke \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"Hello"}]}' | python3 -m json.tool

# Test agent stream endpoint
curl -N -X POST https://ai4d.wiremockapi.cloud/agent/stream \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"Hello"}]}'
```
