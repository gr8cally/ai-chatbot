package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gr8cally/ai-chatbot/internal/agent"
	"github.com/gr8cally/ai-chatbot/internal/config"
	"github.com/gr8cally/ai-chatbot/internal/handler"
	"github.com/gr8cally/ai-chatbot/internal/session"
)

func main() {
	cfg := config.Load()

	store := session.NewStore()
	agentClient := agent.NewClient(cfg.AgentURL)

	pageHandler := handler.NewPageHandler(store)
	chatHandler := handler.NewChatHandler(store, agentClient)

	mux := http.NewServeMux()

	// Pages
	mux.HandleFunc("GET /", pageHandler.HandleIndex)

	// Chat API
	mux.HandleFunc("POST /chat/send", chatHandler.HandleSend)
	mux.HandleFunc("POST /chat/stream/start", chatHandler.HandleStreamStart)
	mux.HandleFunc("GET /chat/stream", chatHandler.HandleStream)
	mux.HandleFunc("POST /chat/abort", chatHandler.HandleAbort)
	mux.HandleFunc("POST /chat/clear", chatHandler.HandleClear)

	// Static files
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Starting server on %s", addr)
	log.Printf("Agent URL: %s", cfg.AgentURL)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
