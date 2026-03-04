package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zeroclaw-labs/goclaw/pkg/agent"
	"github.com/zeroclaw-labs/goclaw/pkg/integrations"
	"github.com/zeroclaw-labs/goclaw/pkg/tools"
	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

type Server struct {
	addr           string
	server         *http.Server
	agent          *agent.Agent
	staticDir      string
	staticFS       http.FileSystem
	sseClients     map[string]chan *types.StreamChunk
	sseClientsMu   sync.RWMutex
	authMiddleware func(http.Handler) http.Handler
	wsClients      map[string]*wsClient
	wsClientsMu    sync.RWMutex
}

type wsClient struct {
	conn     *websocket.Conn
	sendChan chan []byte
}

type Config struct {
	Addr           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxHeaderBytes int
	StaticDir      string // Static files directory
	StaticFS       http.FileSystem
}

type prefixFS struct {
	fs     http.FileSystem
	prefix string
}

func (w *prefixFS) Open(name string) (http.File, error) {
	return w.fs.Open(w.prefix + name)
}

func NewServer(addr string, agent *agent.Agent, staticDir string) *Server {
	return &Server{
		addr:       addr,
		agent:      agent,
		staticDir:  staticDir,
		sseClients: make(map[string]chan *types.StreamChunk),
		wsClients:  make(map[string]*wsClient),
	}
}

func NewServerWithFS(addr string, agent *agent.Agent, staticFS http.FileSystem) *Server {
	return &Server{
		addr:       addr,
		agent:      agent,
		staticFS:   staticFS,
		sseClients: make(map[string]chan *types.StreamChunk),
		wsClients:  make(map[string]*wsClient),
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	
	// Handle API routes FIRST (before static files)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/v1/completions", s.handleCompletions)
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/v1/embeddings", s.handleEmbeddings)
	mux.HandleFunc("/sse", s.handleSSE)
	
	// WebSocket route for agent chat
	mux.HandleFunc("/ws/chat", s.handleWebSocket)
	mux.HandleFunc("/ws", s.handleWebSocket)
	
	// Generic /api/* handler (handles all /api/* paths)
	mux.HandleFunc("/api/", s.handleAPI)
	
	// Serve static files from configured directory or embedded filesystem
	var staticFS http.FileSystem
	
	// Prefer embedded filesystem if provided
	if s.staticFS != nil {
		staticFS = s.staticFS
		log.Printf("Using embedded static files")
	} else if s.staticDir != "" {
		if _, err := os.Stat(s.staticDir); err == nil {
			staticFS = http.Dir(s.staticDir)
			log.Printf("Using static files from: %s", s.staticDir)
		}
	}
	
	if staticFS != nil {
		// Handle static assets under /assets/ - map to web/dist/assets/
		mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(&prefixFS{
			fs:     staticFS,
			prefix: "web/dist/assets",
		})))
		
		// SPA fallback: serve index.html for root and any non-API, non-static paths
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// If it's a file request (has extension), try to serve it
			if filepath.Ext(r.URL.Path) != "" {
				// Try to open the file
				f, err := staticFS.Open("web/dist" + r.URL.Path)
				if err == nil {
					defer f.Close()
					// Determine content type
					contentType := "application/octet-stream"
					switch filepath.Ext(r.URL.Path) {
					case ".html":
						contentType = "text/html; charset=utf-8"
					case ".css":
						contentType = "text/css; charset=utf-8"
					case ".js":
						contentType = "application/javascript; charset=utf-8"
					case ".json":
						contentType = "application/json; charset=utf-8"
					case ".png":
						contentType = "image/png"
					case ".jpg", ".jpeg":
						contentType = "image/jpeg"
					case ".svg":
						contentType = "image/svg+xml"
					}
					w.Header().Set("Content-Type", contentType)
					http.ServeContent(w, r, r.URL.Path, time.Time{}, f.(http.File))
					return
				}
			}
			// Otherwise serve index.html for SPA
			f, err := staticFS.Open("web/dist/index.html")
			if err != nil {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			defer f.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeContent(w, r, "/index.html", time.Time{}, f.(http.File))
		})
		
		log.Printf("Available at: http://localhost%s/", s.addr)
	}

	var handler http.Handler = mux
	if s.authMiddleware != nil {
		handler = s.authMiddleware(mux)
	}

	s.server = &http.Server{
		Addr:         s.addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Printf("Gateway starting on %s", s.addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Gateway error: %v", err)
		}
	}()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Return health status matching ZeroClaw format
	response := map[string]interface{}{
		"status":           "ok",
		"paired":           false,
		"require_pairing":  false,
		"runtime": map[string]interface{}{
			"pid":           os.Getpid(),
			"updated_at":    time.Now().Format(time.RFC3339),
			"uptime_seconds": 0,
			"components": map[string]interface{}{
				"gateway": map[string]interface{}{
					"status":        "ok",
					"updated_at":    time.Now().Format(time.RFC3339),
					"last_ok":       time.Now().Format(time.RFC3339),
					"last_error":    nil,
					"restart_count": 0,
				},
				"daemon": map[string]interface{}{
					"status":        "ok",
					"updated_at":    time.Now().Format(time.RFC3339),
					"last_ok":       time.Now().Format(time.RFC3339),
					"last_error":    nil,
					"restart_count": 0,
				},
				"channels": map[string]interface{}{
					"status":        "ok",
					"updated_at":    time.Now().Format(time.RFC3339),
					"last_ok":       time.Now().Format(time.RFC3339),
					"last_error":    nil,
					"restart_count": 0,
				},
				"scheduler": map[string]interface{}{
					"status":        "ok",
					"updated_at":    time.Now().Format(time.RFC3339),
					"last_ok":       time.Now().Format(time.RFC3339),
					"last_error":    nil,
					"restart_count": 0,
				},
			},
		},
	}
	json.NewEncoder(w).Encode(response)
}

type ChatCompletionRequest struct {
	Model       string              `json:"model"`
	Messages    []types.ChatMessage `json:"messages"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
	Tools       []*types.ToolSpec   `json:"tools,omitempty"`
}

type ChatCompletionResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []ChatChoice      `json:"choices"`
	Usage   *types.TokenUsage `json:"usage,omitempty"`
}

type ChatChoice struct {
	Index        int               `json:"index"`
	Message      types.ChatMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	model := req.Model
	if model == "" {
		model = "gpt-4o"
	}

	temperature := req.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	if req.Stream {
		s.handleStreamingChat(w, r, &req, model, temperature)
		return
	}

	var lastMessage string
	for _, msg := range req.Messages {
		if msg.Role == types.RoleUser {
			lastMessage = msg.Content
		}
	}

	response, err := s.agent.ProcessMessage(r.Context(), lastMessage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	finishReason := "stop"
	if response.HasToolCalls() {
		finishReason = "tool_calls"
	}

	chatResp := ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []ChatChoice{
			{
				Index: 0,
				Message: types.ChatMessage{
					Role:    types.RoleAssistant,
					Content: response.TextOrEmpty(),
				},
				FinishReason: finishReason,
			},
		},
		Usage: response.Usage,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatResp)
}

func (s *Server) handleStreamingChat(w http.ResponseWriter, r *http.Request, req *ChatCompletionRequest, model string, temperature float64) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	clientID := fmt.Sprintf("sse_%d", time.Now().UnixNano())
	ch := make(chan *types.StreamChunk, 10)

	s.sseClientsMu.Lock()
	s.sseClients[clientID] = ch
	s.sseClientsMu.Unlock()

	defer func() {
		s.sseClientsMu.Lock()
		delete(s.sseClients, clientID)
		s.sseClientsMu.Unlock()
		close(ch)
	}()

	go func() {
		var lastMessage string
		for _, msg := range req.Messages {
			if msg.Role == types.RoleUser {
				lastMessage = msg.Content
			}
		}
		_, _ = s.agent.ProcessMessage(r.Context(), lastMessage)
	}()

	for chunk := range ch {
		if chunk.IsFinal {
			break
		}

		data := map[string]interface{}{
			"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   model,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]string{
						"content": chunk.Delta,
					},
				},
			},
		}

		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, "data: %s\n\n", jsonData)
		flusher.Flush()
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *Server) handleCompletions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"message": "completions not implemented, use chat/completions",
			"type":    "invalid_request_error",
		},
	})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{
				"id":       "gpt-4o",
				"object":   "model",
				"created":  1677610602,
				"owned_by": "openai",
			},
		},
	})
}

func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "list",
		"data":   []interface{}{},
	})
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	s.handleChatCompletions(w, r)
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	// Extract the path after /api/
	path := strings.TrimPrefix(r.URL.Path, "/api/")
	
	w.Header().Set("Content-Type", "application/json")
	
	switch path {
	case "status":
		// Return system status matching ZeroClaw format
		response := map[string]interface{}{
			"provider":       "custom:https://ai.gitee.com/v1",
			"model":          "GLM-4.7-Flash",
			"temperature":    0.7,
			"uptime_seconds": 0,
			"gateway_port":   4096,
			"locale":         "zh-CN",
			"memory_backend": "none",
			"paired":         false,
			"channels":       map[string]bool{},
			"health": map[string]interface{}{
				"pid":           os.Getpid(),
				"updated_at":    time.Now().Format(time.RFC3339),
				"uptime_seconds": 0,
				"components": map[string]interface{}{
					"gateway": map[string]interface{}{
						"status":        "ok",
						"updated_at":    time.Now().Format(time.RFC3339),
						"last_ok":       time.Now().Format(time.RFC3339),
						"last_error":    nil,
						"restart_count": 0,
					},
					"daemon": map[string]interface{}{
						"status":        "ok",
						"updated_at":    time.Now().Format(time.RFC3339),
						"last_ok":       time.Now().Format(time.RFC3339),
						"last_error":    nil,
						"restart_count": 0,
					},
					"channels": map[string]interface{}{
						"status":        "ok",
						"updated_at":    time.Now().Format(time.RFC3339),
						"last_ok":       time.Now().Format(time.RFC3339),
						"last_error":    nil,
						"restart_count": 0,
					},
					"scheduler": map[string]interface{}{
						"status":        "ok",
						"updated_at":    time.Now().Format(time.RFC3339),
						"last_ok":       time.Now().Format(time.RFC3339),
						"last_error":    nil,
						"restart_count": 0,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
		return
		
	case "cost":
		// Return cost summary
		response := map[string]interface{}{
			"cost": map[string]interface{}{
				"by_model":         map[string]interface{}{},
				"daily_cost_usd":   0.0,
				"monthly_cost_usd": 0.0,
				"request_count":    0,
				"session_cost_usd": 0.0,
				"total_tokens":     0,
			},
		}
		json.NewEncoder(w).Encode(response)
		return
		
	case "config":
		if r.Method == http.MethodGet {
			// Read actual config file
			configPath := os.ExpandEnv("$HOME/.goclaw/config.toml")
			content, err := os.ReadFile(configPath)
			if err != nil {
				// Return default if config doesn't exist
				response := map[string]interface{}{
					"format":  "toml",
					"content": "# GoClaw Configuration\n# Config file not found\n",
				}
				json.NewEncoder(w).Encode(response)
				return
			}
			
			// Mask sensitive fields
			maskedContent := maskSensitiveFields(string(content))
			
			response := map[string]interface{}{
				"format":  "toml",
				"content": maskedContent,
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
		
	case "tools":
		// Return registered tools from agent
		toolSpecs := s.agent.ToolSpecs()
		toolsList := make([]map[string]interface{}, len(toolSpecs))
		for i, spec := range toolSpecs {
			toolsList[i] = map[string]interface{}{
				"name":        spec.Name,
				"description": spec.Description,
				"parameters":  spec.Parameters,
			}
		}
		response := map[string]interface{}{
			"tools": toolsList,
		}
		json.NewEncoder(w).Encode(response)
		return
		
	case "cron":
		if r.Method == http.MethodGet {
			// Return cron jobs
			response := map[string]interface{}{
				"jobs": []map[string]interface{}{},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
		
	case "integrations":
		// Return all integrations
		response := map[string]interface{}{
			"integrations": integrations.GetAllIntegrations(),
		}
		json.NewEncoder(w).Encode(response)
		return
		
	case "doctor":
		// Return diagnostics
		response := map[string]interface{}{
			"results": []map[string]interface{}{},
			"summary": map[string]int{
				"ok":       0,
				"warnings": 0,
				"errors":   0,
			},
		}
		json.NewEncoder(w).Encode(response)
		return
		
	case "memory":
		if r.Method == http.MethodGet {
			// Return memory entries
			response := map[string]interface{}{
				"entries": []map[string]interface{}{},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
		
	case "cli-tools":
		// Return discovered CLI tools
		cliTools := tools.DiscoverCliTools(nil, nil)
		response := map[string]interface{}{
			"cli_tools": cliTools,
		}
		json.NewEncoder(w).Encode(response)
		return
		
	case "health":
		// Return health snapshot matching ZeroClaw format
		response := map[string]interface{}{
			"health": map[string]interface{}{
				"pid":           os.Getpid(),
				"updated_at":    time.Now().Format(time.RFC3339),
				"uptime_seconds": 0,
				"components": map[string]interface{}{
					"gateway": map[string]interface{}{
						"status":        "ok",
						"updated_at":    time.Now().Format(time.RFC3339),
						"last_ok":       time.Now().Format(time.RFC3339),
						"last_error":    nil,
						"restart_count": 0,
					},
					"daemon": map[string]interface{}{
						"status":        "ok",
						"updated_at":    time.Now().Format(time.RFC3339),
						"last_ok":       time.Now().Format(time.RFC3339),
						"last_error":    nil,
						"restart_count": 0,
					},
					"channels": map[string]interface{}{
						"status":        "ok",
						"updated_at":    time.Now().Format(time.RFC3339),
						"last_ok":       time.Now().Format(time.RFC3339),
						"last_error":    nil,
						"restart_count": 0,
					},
					"scheduler": map[string]interface{}{
						"status":        "ok",
						"updated_at":    time.Now().Format(time.RFC3339),
						"last_ok":       time.Now().Format(time.RFC3339),
						"last_error":    nil,
						"restart_count": 0,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Default response for unknown paths
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleMemoryAPI(w http.ResponseWriter, r *http.Request) {
	// For GET requests, just return success
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
		return
	}
	
	// For POST/PUT/PATCH requests, try to decode the request
	if r.Body == nil || r.ContentLength == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
		return
	}
	
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func (s *Server) SetAuthMiddleware(middleware func(http.Handler) http.Handler) {
	s.authMiddleware = middleware
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// splitProtocols splits a comma-separated Sec-WebSocket-Protocol header value
func splitProtocols(s string) []string {
	var result []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// handleWebSocket handles WebSocket connections for /ws/chat
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("WebSocket request: %s %s", r.Method, r.URL.Path)
	
	// Check for protocol header and respond accordingly
	clientProtocols := r.Header.Get("Sec-WebSocket-Protocol")
	responseHeader := http.Header{}
	if clientProtocols != "" {
		// Accept zeroclaw.v1 protocol if offered
		for _, p := range splitProtocols(clientProtocols) {
			if p == "zeroclaw.v1" {
				responseHeader.Set("Sec-WebSocket-Protocol", "zeroclaw.v1")
				break
			}
		}
	}
	
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	log.Printf("WebSocket upgraded successfully from %s", r.RemoteAddr)

	clientID := fmt.Sprintf("ws_%d", time.Now().UnixNano())
	client := &wsClient{
		conn:     conn,
		sendChan: make(chan []byte, 256),
	}

	s.wsClientsMu.Lock()
	s.wsClients[clientID] = client
	s.wsClientsMu.Unlock()

	defer func() {
		s.wsClientsMu.Lock()
		delete(s.wsClients, clientID)
		s.wsClientsMu.Unlock()
		conn.Close()
	}()

	// Send welcome message
	welcome := map[string]interface{}{
		"type":    "connected",
		"message": "WebSocket connected successfully",
	}
	if data, err := json.Marshal(welcome); err == nil {
		conn.WriteMessage(websocket.TextMessage, data)
	}

	// Read messages from client
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		log.Printf("Received WebSocket message: %s", string(message))

		// Parse message
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("WebSocket message parse error: %v", err)
			continue
		}

		log.Printf("Parsed message: %+v", msg)

		msgType, _ := msg["type"].(string)
		
		switch msgType {
		case "message", "chat":
			// Handle chat message
			content, _ := msg["content"].(string)
			if content == "" {
				// Try to get content from data field
				if data, ok := msg["data"].(map[string]interface{}); ok {
					content, _ = data["content"].(string)
				}
			}
			
			if content == "" {
				log.Printf("Empty message content")
				continue
			}

			sessionID, _ := msg["session_id"].(string)
			if sessionID == "" {
				sessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
			}

			// Call agent
			go s.handleAgentChat(conn, sessionID, content)

		case "ping":
			pong := map[string]interface{}{
				"type":      "pong",
				"timestamp": time.Now().Unix(),
			}
			if data, err := json.Marshal(pong); err == nil {
				conn.WriteMessage(websocket.TextMessage, data)
			}

		default:
			// Echo back for unknown types
			response := map[string]interface{}{
				"type":    "response",
				"message": "Message received",
				"data":    msg,
			}
			if data, err := json.Marshal(response); err == nil {
				conn.WriteMessage(websocket.TextMessage, data)
			}
		}
	}
}

// handleAgentChat processes a chat message through the agent
func (s *Server) handleAgentChat(conn *websocket.Conn, sessionID, content string) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	log.Printf("Processing chat message: %s", content)

	response, err := s.agent.ProcessMessage(ctx, content)
	if err != nil {
		log.Printf("Agent error: %v", err)
		errMsg := map[string]interface{}{
			"type":      "error",
			"session_id": sessionID,
			"error":      err.Error(),
		}
		if data, err := json.Marshal(errMsg); err == nil {
			conn.WriteMessage(websocket.TextMessage, data)
		}
		return
	}

	// Send response
	resp := map[string]interface{}{
		"type":          "done",
		"session_id":    sessionID,
		"content":       response.TextOrEmpty(),
		"full_response": response.TextOrEmpty(),
		"finish_reason": "stop",
	}
	if response.Usage != nil {
		resp["usage"] = response.Usage
	}

	if data, err := json.Marshal(resp); err == nil {
		conn.WriteMessage(websocket.TextMessage, data)
		log.Printf("Sent response for session %s", sessionID)
	}
}

// maskSensitiveFields masks sensitive fields in config content
func maskSensitiveFields(content string) string {
	// List of sensitive field patterns to mask
	sensitivePatterns := []struct {
		pattern string
		replacement string
	}{
		{`api_key\s*=\s*"[^"]*"`, `api_key = "***MASKED***"`},
		{`api_key\s*=\s*'[^']*'`, `api_key = '***MASKED***'`},
		{`client_secret\s*=\s*"[^"]*"`, `client_secret = "***MASKED***"`},
		{`client_secret\s*=\s*'[^']*'`, `client_secret = '***MASKED***'`},
		{`token\s*=\s*"[^"]*"`, `token = "***MASKED***"`},
		{`token\s*=\s*'[^']*'`, `token = '***MASKED***'`},
		{`secret\s*=\s*"[^"]*"`, `secret = "***MASKED***"`},
		{`secret\s*=\s*'[^']*'`, `secret = '***MASKED***'`},
		{`password\s*=\s*"[^"]*"`, `password = "***MASKED***"`},
		{`password\s*=\s*'[^']*'`, `password = '***MASKED***'`},
		{`api_keys\s*=\s*\[[^\]]*\]`, `api_keys = ["***MASKED***"]`},
	}
	
	result := content
	for _, sp := range sensitivePatterns {
		re := regexp.MustCompile(sp.pattern)
		result = re.ReplaceAllString(result, sp.replacement)
	}
	
	return result
}
