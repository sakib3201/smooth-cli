package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"

	"github.com/smoothcli/smooth-cli/internal/events"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type SupervisorReader interface {
	State(name string) (map[string]interface{}, error)
	States() map[string]map[string]interface{}
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Restart(ctx context.Context, name string) error
	SendInput(ctx context.Context, name string, input []byte) error
}

type LogReader interface {
	Lines(process string, n int) []interface{}
}

type StateReader interface {
	AllProcesses() map[string]map[string]interface{}
}

type ConfigReader interface {
	GetConfig() interface{}
}

type BusSubscriber interface {
	Subscribe() (<-chan events.Event, func())
}

type Deps struct {
	Supervisor SupervisorReader
	LogStore   LogReader
	State      StateReader
	Config     ConfigReader
	Bus        BusSubscriber
}

type Server struct {
	srv     *http.Server
	handler http.Handler
	deps    Deps
}

func New(port int, deps Deps) *Server {
	s := &Server{deps: deps}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(jsonContentType)

	r.Get("/api/v1/processes", s.handleListProcesses)
	r.Get("/api/v1/processes/{name}", s.handleGetProcess)
	r.Post("/api/v1/processes/{name}/start", s.handleStart)
	r.Post("/api/v1/processes/{name}/stop", s.handleStop)
	r.Post("/api/v1/processes/{name}/restart", s.handleRestart)
	r.Post("/api/v1/processes/{name}/input", s.handleSendInput)
	r.Get("/api/v1/processes/{name}/logs", s.handleLogs)
	r.Get("/api/v1/config", s.handleGetConfig)
	r.Post("/api/v1/config/reload", s.handleReloadConfig)
	r.Get("/api/v1/events", s.handleEventHistory)
	r.Get("/api/v1/health", s.handleHealth)
	r.Get("/ws", s.handleWebSocket)

	s.handler = r
	s.srv = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: r,
	}
	return s
}

func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("api server listen: %w", err)
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(shutCtx)
	}()
	if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("api server: %w", err)
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok", "version": "1.0.0"})
}

func (s *Server) handleListProcesses(w http.ResponseWriter, r *http.Request) {
	states := s.deps.Supervisor.States()
	writeJSON(w, states)
}

func (s *Server) handleGetProcess(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	_, err := s.deps.Supervisor.State(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "process not found")
		return
	}
	state, _ := s.deps.Supervisor.State(name)
	writeJSON(w, state)
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	err := s.deps.Supervisor.Start(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "started"})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	err := s.deps.Supervisor.Stop(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "stopped"})
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	err := s.deps.Supervisor.Restart(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "restarted"})
}

func (s *Server) handleSendInput(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if len(body.Text) > 10240 {
		writeError(w, http.StatusBadRequest, "text exceeds 10KB")
		return
	}
	err := s.deps.Supervisor.SendInput(r.Context(), name, []byte(body.Text))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "sent"})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	lines := s.deps.LogStore.Lines(name, 100)
	writeJSON(w, lines)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.deps.Config.GetConfig()
	writeJSON(w, cfg)
}

func (s *Server) handleReloadConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "reloaded"})
}

func (s *Server) handleEventHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, []interface{}{})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	go s.wsClient(conn)
}

func (s *Server) wsClient(conn *websocket.Conn) {
	defer conn.Close()
	ch, unsub := s.deps.Bus.Subscribe()
	defer unsub()
	for ev := range ch {
		data, _ := json.Marshal(map[string]interface{}{
			"kind":      ev.Kind,
			"timestamp": ev.Timestamp,
		})
		conn.WriteJSON(data)
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
