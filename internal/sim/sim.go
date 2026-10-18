package sim

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rohitshukla001/Kinhaven/internal/agent"
)

//go:embed web/index.html
var indexHTML []byte

const (
	maxSessions   = 50
	maxHistory    = 40
	maxQuestion   = 500
	maxBodyBytes  = 64 << 10
	uiURIPrefix   = "ui://"
	jsonMediaType = "application/json"
)

type Server struct {
	model   string
	agent   *agent.Agent
	mcp     *mcp.ClientSession
	logger  *slog.Logger
	tools   map[string]bool
	mu      sync.Mutex
	byID    map[string]*conversation
	ordered []string
}

type conversation struct {
	mu      sync.Mutex
	history []agent.Message
}

func New(model string, a *agent.Agent, session *mcp.ClientSession, logger *slog.Logger) *Server {
	s := &Server{model: model, agent: a, mcp: session, logger: logger, tools: map[string]bool{}, byID: map[string]*conversation{}}
	for _, t := range a.Tools() {
		s.tools[t.Name] = true
	}
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.index)
	mux.Handle("GET /api/info", sameOrigin(http.HandlerFunc(s.info)))
	mux.Handle("POST /api/ask", sameOrigin(http.HandlerFunc(s.ask)))
	mux.Handle("POST /api/tool", sameOrigin(http.HandlerFunc(s.callTool)))
	mux.Handle("GET /api/resource", sameOrigin(http.HandlerFunc(s.readResource)))
	return mux
}

func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host {
			writeError(w, http.StatusForbidden, "origin not allowed")
			return
		}
		if r.Method == http.MethodPost && !strings.HasPrefix(r.Header.Get("Content-Type"), jsonMediaType) {
			writeError(w, http.StatusUnsupportedMediaType, "send JSON")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src 'self' data:; frame-src 'self' about: data:; connect-src 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(indexHTML)
}

func (s *Server) info(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"model": s.model, "tools": len(s.tools)})
}

type askRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

type askResponse struct {
	SessionID string       `json:"session_id"`
	Reply     string       `json:"reply"`
	Calls     []agent.Call `json:"calls"`
}

func (s *Server) ask(w http.ResponseWriter, r *http.Request) {
	var req askRequest
	if !decode(w, r, &req) {
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" || len(text) > maxQuestion {
		writeError(w, http.StatusBadRequest, "text must be 1 to 500 characters")
		return
	}

	id, conv := s.conversation(req.SessionID)
	conv.mu.Lock()
	defer conv.mu.Unlock()

	history, turn, err := s.agent.Ask(r.Context(), conv.history, text)
	if err != nil {
		s.logger.Warn("assistant turn failed", "err", err)
		writeError(w, http.StatusBadGateway, "the assistant could not answer: "+err.Error())
		return
	}
	conv.history = trim(history, maxHistory)
	if turn.Calls == nil {
		turn.Calls = []agent.Call{}
	}
	writeJSON(w, http.StatusOK, askResponse{SessionID: id, Reply: turn.Reply, Calls: turn.Calls})
}

type toolRequest struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func (s *Server) callTool(w http.ResponseWriter, r *http.Request) {
	var req toolRequest
	if !decode(w, r, &req) {
		return
	}
	if !s.tools[req.Name] {
		writeError(w, http.StatusNotFound, "unknown tool "+req.Name)
		return
	}
	res, err := s.mcp.CallTool(r.Context(), &mcp.CallToolParams{Name: req.Name, Arguments: req.Arguments})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) readResource(w http.ResponseWriter, r *http.Request) {
	uri := r.URL.Query().Get("uri")
	if !strings.HasPrefix(uri, uiURIPrefix) {
		writeError(w, http.StatusBadRequest, "only ui:// resources can be shown")
		return
	}
	res, err := s.mcp.ReadResource(r.Context(), &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if len(res.Contents) == 0 {
		writeError(w, http.StatusNotFound, "resource has no contents")
		return
	}
	writeJSON(w, http.StatusOK, res.Contents[0])
}

func (s *Server) conversation(id string) (string, *conversation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.byID[id]; ok {
		return id, c
	}
	if len(s.ordered) >= maxSessions {
		delete(s.byID, s.ordered[0])
		s.ordered = s.ordered[1:]
	}
	b := make([]byte, 12)
	rand.Read(b)
	id = hex.EncodeToString(b)
	c := &conversation{}
	s.byID[id] = c
	s.ordered = append(s.ordered, id)
	return id, c
}

func trim(history []agent.Message, limit int) []agent.Message {
	if len(history) <= limit {
		return history
	}
	start := slices.IndexFunc(history[len(history)-limit:], func(m agent.Message) bool {
		return m.Role == agent.User && len(m.Parts) > 0 && m.Parts[0].Text != ""
	})
	if start < 0 {
		return nil
	}
	return slices.Clone(history[len(history)-limit+start:])
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "request is too large")
		} else {
			writeError(w, http.StatusBadRequest, "request body must be JSON")
		}
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", jsonMediaType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
