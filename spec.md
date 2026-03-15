# AI Chatbot Interface — Technical Specification

## 1. Overview

This document specifies a production-quality AI chatbot interface that connects to a LangChain Agent backend. The application is built with **Go** (HTTP server + backend proxy), **Templ** (type-safe HTML templating), **HTMX** (declarative browser interactivity), and **TypeScript** (streaming animation, markdown rendering, clipboard).

The chatbot sends user prompts to a remote agent endpoint and renders responses with modern AI UX: streaming tokens, markdown with syntax-highlighted code blocks, copy-to-clipboard, abort generation, and graceful error handling.

---

## 2. System Architecture

```
┌──────────────────────────────────────────────────────────┐
│                      Browser                             │
│                                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │   HTMX       │  │  TypeScript  │  │  CSS           │  │
│  │  (requests,  │  │  (streaming  │  │  (layout,      │  │
│  │   swaps,     │  │   animation, │  │   bubbles,     │  │
│  │   SSE)       │  │   markdown,  │  │   responsive)  │  │
│  │              │  │   clipboard) │  │               │  │
│  └──────┬───────┘  └──────┬───────┘  └───────────────┘  │
│         │                 │                              │
└─────────┼─────────────────┼──────────────────────────────┘
          │ HTTP / SSE      │
          ▼                 │
┌──────────────────────────────────────────────────────────┐
│                    Go HTTP Server                         │
│                                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │ Templ        │  │ Chat Handler │  │ SSE Stream     │  │
│  │ Templates    │  │ (invoke)     │  │ Handler        │  │
│  │ (HTML frags) │  │              │  │ (stream proxy) │  │
│  └──────────────┘  └──────┬───────┘  └──────┬────────┘  │
│                           │                  │           │
│                    ┌──────┴──────────────────┴────┐      │
│                    │   Session Store (in-memory)  │      │
│                    │   map[sessionID][]Message     │      │
│                    └──────┬──────────────────────┘      │
│                           │                              │
└───────────────────────────┼──────────────────────────────┘
                            │ HTTP POST
                            ▼
              ┌──────────────────────────┐
              │  LangChain Agent API     │
              │  /agent/invoke           │
              │  /agent/stream           │
              └──────────────────────────┘
```

### Key Architectural Decisions

| Decision | Rationale |
|---|---|
| Go as BFF (backend-for-frontend) | Proxies agent API calls, keeps API URL server-side, manages session state, re-emits SSE for the browser |
| Templ for HTML | Type-safe, compile-time checked templates; generates Go code; no runtime template parsing |
| HTMX for interactivity | Declarative HTTP requests, SSE support via `hx-ext="sse"`, HTML fragment swapping — minimal JS needed |
| TypeScript for progressive enhancement | Handles streaming animation, markdown rendering, code highlighting, clipboard — things that need imperative DOM manipulation |
| Server-side session store | Chat history lives on the server (in-memory map), simplifying the frontend and matching the agent API's `messages` array contract |
| SSE for streaming | The Go server consumes the agent's SSE stream and re-emits it to the browser. HTMX's SSE extension handles connection lifecycle |

---

## 3. Agent API Contract

The chatbot integrates with a LangChain Agent at a configurable base URL.

### 3.1 Environment Variable

```
AGENT_URL=https://ai4d.wiremockapi.cloud
```

All agent requests are made **server-side** from the Go backend. The browser never calls the agent directly.

### 3.2 POST `/agent/invoke` (Non-streaming)

**Request:**
```json
{
  "messages": [
    { "role": "user", "content": "What is attention?" },
    { "role": "assistant", "content": "Attention is..." },
    { "role": "user", "content": "Explain more" }
  ]
}
```

The `messages` field must match the regex `\{\s*"messages"\:.*` (the WireMock stub pattern).

**Response (200 OK):**
```json
{
  "output": {
    "messages": [
      {
        "content": "What is attention?",
        "type": "human",
        "id": "0"
      },
      {
        "content": "**Attention** is a mechanism that...",
        "type": "ai",
        "id": "1",
        "tool_calls": [],
        "usage_metadata": { ... }
      }
    ],
    "structured_response": null
  }
}
```

**Key fields:**
- `output.messages` — array of all messages in the conversation (human + ai)
- Each message has `type` (`"human"` or `"ai"`) and `content` (string, may contain markdown)
- The **last message with `type: "ai"`** is the agent's response to display

### 3.3 POST `/agent/stream` (Streaming — SSE)

**Request:** Same JSON body as `/agent/invoke`.

**Response:** `text/event-stream` with the following event types:

| Event | Data Structure | Purpose |
|---|---|---|
| `message_start` | `{"type":"message_start","message":{"id":"0","role":"human","content":"..."}}` | Echoes back each message in the conversation history |
| `content_block_start` | `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` | Signals start of a new content block |
| `content_block_delta` | `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"..."}}` | **Incremental text chunk** — the primary streaming payload |
| `content_block_stop` | `{"type":"content_block_stop","index":0}` | Signals end of current content block |
| `message_delta` | `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{...}}` | Metadata: stop reason, token usage |
| `message_stop` | `{"type":"message_stop","message":{"id":"3","role":"assistant","stop_reason":"end_turn"}}` | Signals conversation turn is complete |

**Streaming strategy:**
1. Accumulate `content_block_delta` events to build the response text
2. The **last `message_stop` with `role: "assistant"`** marks the final response
3. Prior `message_start` events with `role: "human"` are conversation history echoes (ignore for display)

### 3.4 Error Handling

The mock API may return:
- **Non-matching request body** → WireMock returns a detailed mismatch error (not JSON)
- **Network errors** → timeout, DNS failure, connection refused
- **5xx errors** → server-side failures

All error cases must be handled gracefully in the UI.

---

## 4. Data Model

### 4.1 Message

```go
type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
)

type Message struct {
    ID        string    `json:"id"`
    Role      Role      `json:"role"`
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
}
```

### 4.2 Session

```go
type Session struct {
    ID        string
    Messages  []Message
    CreatedAt time.Time
    mu        sync.RWMutex
}
```

### 4.3 Session Store

```go
type SessionStore struct {
    sessions map[string]*Session
    mu       sync.RWMutex
}
```

- Sessions are keyed by a UUID stored in a browser cookie (`session_id`)
- The store is in-memory; sessions are lost on server restart (acceptable for this scope)
- Thread-safe access via `sync.RWMutex`

---

## 5. Go Server Endpoints

| Method | Path | Purpose | Response |
|---|---|---|---|
| `GET` | `/` | Serve the main chat page | Full HTML page (Templ) |
| `POST` | `/chat/send` | Non-streaming: send message, get response | HTML fragment (new message bubbles) via HTMX swap |
| `GET` | `/chat/stream` | SSE endpoint: proxy agent stream to browser | `text/event-stream` |
| `POST` | `/chat/stream/start` | Initiate a streaming request | Triggers SSE connection setup, returns HTML fragment with SSE listener |
| `POST` | `/chat/abort` | Cancel an in-flight streaming request | 200 OK; cancels the upstream context |
| `POST` | `/chat/clear` | Clear conversation history | HTML fragment (empty chat area) |
| `GET` | `/static/*` | Serve static assets (CSS, JS, images) | Static files |

### 5.1 Request Flow (Streaming)

1. User submits message via form
2. HTMX `POST /chat/stream/start` with `hx-vals='{"message": "..."}'`
3. Server: appends user message to session, returns HTML fragment containing:
   - User message bubble
   - Assistant bubble placeholder with `hx-ext="sse"` pointing to `/chat/stream?session_id=...`
4. Browser opens SSE connection to `/chat/stream`
5. Server: calls agent `/agent/stream` with full message history, re-emits `content_block_delta` text as SSE events
6. TypeScript: receives SSE events, appends text to assistant bubble, renders markdown incrementally
7. On `message_stop`: server sends final SSE event, closes connection; TS finalizes markdown rendering
8. Server: appends complete assistant message to session history

### 5.2 Request Flow (Non-streaming Fallback)

1. User submits message via form
2. HTMX `POST /chat/send` with message content
3. Server: appends user message to session, calls agent `/agent/invoke`, extracts last AI message
4. Server: appends assistant message to session, returns HTML fragment with both bubbles
5. HTMX swaps fragment into chat container

---

## 6. Frontend Components (Templ)

### 6.1 Page Layout — `layout.templ`

```
<!DOCTYPE html>
<html>
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>AI Chat</title>
    <link rel="stylesheet" href="/static/css/styles.css">
    <script src="/static/js/htmx.min.js"></script>
    <script src="/static/js/htmx-sse.js"></script>
    <script src="/static/js/app.js" type="module"></script>
  </head>
  <body>
    { children... }
  </body>
</html>
```

### 6.2 Chat Page — `chat.templ`

```
┌─────────────────────────────────┐
│  Header: "AI Chat"    [Clear]   │
├─────────────────────────────────┤
│                                 │
│  ┌─────────────────────────┐    │
│  │ User bubble (right)     │    │
│  └─────────────────────────┘    │
│                                 │
│  ┌─────────────────────────┐    │
│  │ Assistant bubble (left)  │   │
│  │ (markdown rendered)      │   │
│  └─────────────────────────┘    │
│                                 │
│  ... more messages ...          │
│                                 │
│  ┌─────────────────────────┐    │
│  │ Typing indicator (dots)  │   │  ← shown during generation
│  └─────────────────────────┘    │
│                                 │
├─────────────────────────────────┤
│  [text input          ] [Send]  │
│                        [Abort]  │  ← shown during generation
└─────────────────────────────────┘
```

### 6.3 Message Bubble — `message.templ`

- User messages: right-aligned, distinct background color
- Assistant messages: left-aligned, different background color, content rendered as markdown
- Each bubble shows a timestamp
- Assistant bubbles have a copy button (copies raw markdown)

### 6.4 Streaming Placeholder — `stream_placeholder.templ`

- An empty assistant bubble with:
  - `id="stream-target"` for JS to append text into
  - `hx-ext="sse"` and `sse-connect="/chat/stream?sid=..."` for HTMX SSE
  - A pulsing cursor animation

---

## 7. TypeScript Modules

### 7.1 `streaming.ts` — SSE Client & Animation

**Responsibilities:**
- Listen for SSE `content_block_delta` events
- Buffer incoming text chunks
- Animate text appearance (character-by-character or chunk-by-chunk with configurable speed)
- On stream end: finalize the content, trigger markdown rendering

**Exported interface:**
```typescript
interface StreamConfig {
  targetElementId: string;
  sseUrl: string;
  onChunk: (text: string, accumulated: string) => void;
  onComplete: (fullText: string) => void;
  onError: (error: Error) => void;
}

function startStream(config: StreamConfig): AbortController;
```

### 7.2 `markdown.ts` — Markdown Rendering

**Responsibilities:**
- Render markdown to HTML using `marked` (or `markdown-it`)
- Apply syntax highlighting to code blocks using `highlight.js`
- Sanitize HTML output (DOMPurify)
- Support incremental re-rendering during streaming (debounced, every ~100ms)

**Exported interface:**
```typescript
function renderMarkdown(raw: string): string;
function renderMarkdownIncremental(element: HTMLElement, raw: string): void;
```

### 7.3 `clipboard.ts` — Copy to Clipboard

**Responsibilities:**
- Attach click handlers to `.copy-btn` elements on code blocks
- Use `navigator.clipboard.writeText()` with fallback
- Show brief "Copied!" feedback

### 7.4 `autoscroll.ts` — Auto-scroll Management

**Responsibilities:**
- Auto-scroll chat container to bottom on new messages
- Detect manual scroll-up and pause auto-scroll
- Resume auto-scroll when user scrolls back to bottom

---

## 8. CSS / Styling

### 8.1 Approach

Plain CSS with CSS custom properties (variables) for theming. No CSS framework dependency.

### 8.2 Layout

- Full-viewport height chat layout using CSS Grid or Flexbox:
  - Header (fixed height)
  - Messages area (scrollable, `flex: 1` / `overflow-y: auto`)
  - Input area (fixed height)
- Responsive: works on mobile (min-width ~320px) and desktop
- Max-width container for readability (~768px centered)

### 8.3 Message Bubbles

- User: right-aligned, colored background (e.g., blue), white text, rounded corners
- Assistant: left-aligned, light background, dark text, rounded corners
- Code blocks: dark background, monospace font, horizontal scroll for overflow, copy button positioned top-right

### 8.4 States

- **Loading/generating**: pulsing dots animation in assistant bubble, input disabled, send button replaced with abort button
- **Error**: red-tinted error message bubble with retry option
- **Empty state**: centered welcome message with suggested prompts

---

## 9. Configuration & Environment

### 9.1 Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `AGENT_URL` | Yes | — | Base URL of the LangChain agent API (e.g., `https://ai4d.wiremockapi.cloud`) |
| `PORT` | No | `8080` | HTTP server listen port |
| `SESSION_SECRET` | No | random | Secret for signing session cookies |

### 9.2 `.env-example`

```env
# Required: Base URL for the LangChain Agent API
AGENT_URL=https://ai4d.wiremockapi.cloud

# Optional: Server port (default: 8080)
PORT=8080

# Optional: Session cookie signing secret
SESSION_SECRET=change-me-in-production
```

Note: The requirement mentions `NEXT_PUBLIC_AGENT_URL` but since we are not using Next.js, we use `AGENT_URL` as a server-side env var. The API URL is **never exposed to the browser** — the Go server proxies all agent requests.

---

## 10. Dependencies

### 10.1 Go Modules

| Module | Purpose |
|---|---|
| `github.com/a-h/templ` | HTML templating |
| `github.com/joho/godotenv` | Load `.env` files |
| `github.com/google/uuid` | Session ID generation |
| Standard library (`net/http`, `encoding/json`, `bufio`, etc.) | HTTP server, SSE parsing, JSON handling |

### 10.2 Frontend (vendored or CDN)

| Library | Purpose | Delivery |
|---|---|---|
| htmx (~14KB gzipped) | Declarative HTTP + SSE | Vendored in `/static/js/` |
| htmx SSE extension | SSE support for htmx | Vendored in `/static/js/` |
| marked (~7KB gzipped) | Markdown → HTML | ESM import or vendored |
| highlight.js (~30KB core + languages) | Syntax highlighting | ESM import or vendored |
| DOMPurify (~7KB gzipped) | HTML sanitization | ESM import or vendored |

### 10.3 Build Tools

| Tool | Purpose |
|---|---|
| `templ generate` | Compile `.templ` files to Go code |
| `esbuild` or `tsc` | Compile TypeScript to JavaScript |
| `air` (optional) | Hot-reload Go server during development |

---

## 11. Non-Functional Requirements

| Requirement | Target |
|---|---|
| First contentful paint | < 500ms (server-rendered HTML, minimal JS) |
| Time to interactive | < 1s |
| Streaming latency (first token visible) | < 200ms after SSE connection opens |
| Mobile responsive | 320px – 1920px+ |
| Accessibility | Semantic HTML, ARIA labels on interactive elements, keyboard-navigable |
| Browser support | Modern evergreen browsers (Chrome, Firefox, Safari, Edge — last 2 versions) |
| Security | No XSS (DOMPurify), no agent URL leaked to client, session cookie HttpOnly+Secure |

---

## 12. Out of Scope

- User authentication / login
- Persistent storage (database)
- Multi-user / multi-tenant
- File uploads or image generation
- Agent tool-call rendering (tool_calls are present in API but not displayed)
- Dark mode (could be added as a stretch goal)
- Unit / integration / E2E tests (could be added as a stretch goal)
- Deployment / CI/CD pipeline
