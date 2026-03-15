# AI Chatbot — Go, Templ, HTMX, and SSE

A production-quality AI chatbot interface that connects to a LangChain Agent backend. This application demonstrates a modern AI UX with streaming tokens, markdown rendering, syntax highlighting, and graceful error handling.

## 🚀 How to Run

### Prerequisites

- **Go**: >= 1.23
- **Templ**: >= 0.2.x (`go install github.com/a-h/templ/cmd/templ@latest`)
- **Node.js**: >= 20 (for TypeScript compilation)

### Quick Start

1.  **Clone the repository and set up environment variables**:
    ```bash
    cp .env-example .env
    # Edit .env and set your AGENT_URL (e.g., https://ai4d.wiremockapi.cloud)
    ```

2.  **Build and Run**:
    ```bash
    make build
    make run
    ```
    The application will be available at `http://localhost:8080`.

## 📂 Repository Summary

This project implements a **Backend-for-Frontend (BFF)** architecture using Go to proxy requests to an upstream AI agent.

### Core Technologies
- **Go 1.23**: High-performance backend server and SSE proxy.
- **Templ**: Type-safe HTML templating for Go.
- **HTMX**: Declarative browser interactivity and SSE connection management.
- **TypeScript**: Progressive enhancement for streaming animations, markdown rendering (via `marked`), and code highlighting (via `highlight.js`).
- **Server-Sent Events (SSE)**: Used for the "streaming tokens" experience.

### Architecture Overview
- **Go Server**: Manages user sessions (in-memory), proxies streaming responses from the agent API to the browser, and serves static assets.
- **Frontend**: A thin layer of HTMX for DOM swaps and TypeScript for complex UI logic like incremental markdown rendering and auto-scrolling.
- **Session Management**: Uses an HTTP-only cookie to track conversation history in a thread-safe in-memory store.

### Key Features
- **Real-time Streaming**: Assistant responses appear token-by-token as they are generated.
- **Markdown Support**: Rich text rendering including headings, lists, and links.
- **Code Blocks**: Syntax highlighting with a "Copy to Clipboard" button.
- **Abort Generation**: Ability to stop an in-flight AI response.
- **Responsive UI**: Mobile-friendly design with auto-scrolling and manual scroll detection.
- **Error Handling**: Graceful recovery from network failures or API errors.

## 🛠️ Development

To run with hot-reload (requires [Air](https://github.com/air-verse/air)):

```bash
make dev
```

### Build Commands (Makefile)
- `make generate`: Generates Go code from `.templ` files.
- `make ts`: Compiles and minifies TypeScript to `static/js/dist/app.js`.
- `make build`: Performs both generation and Go compilation.
- `make run`: Builds and executes the binary.
- `make clean`: Removes build artifacts and `node_modules`.

## ⚙️ Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `AGENT_URL` | Yes | — | Base URL of the LangChain agent API |
| `PORT` | No | `8080` | HTTP server listen port |
| `SESSION_SECRET` | No | random | Secret for signing session cookies |

---

For technical specifications, see [spec.md](./spec.md). For the detailed implementation plan, see [implementation.md](./implementation.md).
