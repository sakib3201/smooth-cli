package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/smoothcli/smooth-cli/internal/domain"
)

var (
	ErrProcessNotFound = errors.New("process not found")
	ErrRequestNotFound = errors.New("permission request not found")
)

type Store interface {
	SetProcess(state domain.ProcessState)
	GetProcess(name string) (domain.ProcessState, error)
	AllProcesses() map[string]domain.ProcessState
	AddAttentionEvent(e domain.AttentionEvent)
	ResolveAttention(process string)
	ActiveAttentionEvents() []domain.AttentionEvent
	AddPermissionRequest(req domain.PermissionRequest)
	ResolvePermissionRequest(id string, approved bool) error
	PendingPermissionRequests() []domain.PermissionRequest
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
	Close() error
}

type MemStore struct {
	mu                 sync.RWMutex
	processes          map[string]domain.ProcessState
	attentionEvents    []domain.AttentionEvent
	permissionRequests map[string]domain.PermissionRequest
	settings           map[string]string
	db                 *sql.DB
}

func New(db *sql.DB) *MemStore {
	s := &MemStore{
		processes:          make(map[string]domain.ProcessState),
		permissionRequests: make(map[string]domain.PermissionRequest),
		settings:           make(map[string]string),
		db:                 db,
	}
	s.loadFromDB()
	return s
}

func (s *MemStore) loadFromDB() {
	if s.db == nil {
		return
	}

	rows, err := s.db.Query("SELECT name, config_json, last_status, last_pid, restart_count, updated_at FROM processes")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name, configJSON, lastStatus string
		var lastPID, restartCount sql.NullInt64
		var updatedAt string
		if err := rows.Scan(&name, &configJSON, &lastStatus, &lastPID, &restartCount, &updatedAt); err != nil {
			continue
		}
		var state domain.ProcessState
		if err := json.Unmarshal([]byte(configJSON), &state); err == nil {
			if lastPID.Valid {
				state.PID = int(lastPID.Int64)
			}
			state.RestartCount = int(restartCount.Int64)
			s.processes[name] = state
		}
	}

	rows2, err := s.db.Query("SELECT id, process, action, old_value, new_value, requested_at, expires_at, approved, decided_at FROM permission_requests WHERE approved IS NULL")
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var id, process, action, newValue, requestedAt, expiresAt string
			var oldValue, decidedAt sql.NullString
			var approved sql.NullInt64
			if err := rows2.Scan(&id, &process, &action, &oldValue, &newValue, &requestedAt, &expiresAt, &approved, &decidedAt); err != nil {
				continue
			}
			req := domain.PermissionRequest{
				ID:       id,
				Process:  process,
				Action:   action,
				NewValue: newValue,
			}
			if oldValue.Valid {
				req.OldValue = oldValue.String
			}
			if t, err := time.Parse(time.RFC3339, requestedAt); err == nil {
				req.RequestedAt = t
			}
			if t, err := time.Parse(time.RFC3339, expiresAt); err == nil {
				req.ExpiresAt = t
			}
			s.permissionRequests[id] = req
		}
	}

	rows3, err := s.db.Query("SELECT key, value FROM settings")
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var key, value string
			if err := rows3.Scan(&key, &value); err == nil {
				s.settings[key] = value
			}
		}
	}

	rows4, err := s.db.Query("SELECT process, pattern, context, occurred_at, resolved FROM attention_events WHERE resolved=0")
	if err == nil {
		defer rows4.Close()
		for rows4.Next() {
			var process, pattern, context, occurredAt string
			var resolved int
			if err := rows4.Scan(&process, &pattern, &context, &occurredAt, &resolved); err == nil {
				e := domain.AttentionEvent{
					Process:  process,
					Pattern:  pattern,
					Context:  context,
					Resolved: resolved == 1,
				}
				if t, err := time.Parse(time.RFC3339, occurredAt); err == nil {
					e.Timestamp = t
				}
				s.attentionEvents = append(s.attentionEvents, e)
			}
		}
	}
}

func (s *MemStore) SetProcess(state domain.ProcessState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.processes[state.Name] = state

	if s.db != nil {
		configJSON, _ := json.Marshal(state)
		updatedAt := time.Now().UTC().Format(time.RFC3339)
		lastStatus := string(state.Status)
		var lastPID interface{} = nil
		if state.PID > 0 {
			lastPID = state.PID
		}
		s.db.Exec(`INSERT INTO processes (name, config_json, last_status, last_pid, restart_count, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET
				config_json=excluded.config_json,
				last_status=excluded.last_status,
				last_pid=excluded.last_pid,
				restart_count=excluded.restart_count,
				updated_at=excluded.updated_at`,
			state.Name, string(configJSON), lastStatus, lastPID, state.RestartCount, updatedAt)
	}
}

func (s *MemStore) GetProcess(name string) (domain.ProcessState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.processes[name]
	if !ok {
		return domain.ProcessState{}, ErrProcessNotFound
	}
	return state, nil
}

func (s *MemStore) AllProcesses() map[string]domain.ProcessState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]domain.ProcessState, len(s.processes))
	for k, v := range s.processes {
		result[k] = v
	}
	return result
}

func (s *MemStore) AddAttentionEvent(e domain.AttentionEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.attentionEvents = append(s.attentionEvents, e)

	if s.db != nil {
		occurredAt := e.Timestamp.UTC().Format(time.RFC3339)
		s.db.Exec(`INSERT INTO attention_events (process, pattern, context, occurred_at, resolved) VALUES (?, ?, ?, ?, 0)`,
			e.Process, e.Pattern, e.Context, occurredAt)
	}
}

func (s *MemStore) ResolveAttention(process string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.attentionEvents {
		if s.attentionEvents[i].Process == process {
			s.attentionEvents[i].Resolved = true
		}
	}

	if s.db != nil {
		s.db.Exec(`UPDATE attention_events SET resolved=1 WHERE process=? AND resolved=0`, process)
	}
}

func (s *MemStore) ActiveAttentionEvents() []domain.AttentionEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var active []domain.AttentionEvent
	for i := len(s.attentionEvents) - 1; i >= 0; i-- {
		if !s.attentionEvents[i].Resolved {
			active = append(active, s.attentionEvents[i])
		}
	}
	return active
}

func (s *MemStore) AddPermissionRequest(req domain.PermissionRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.permissionRequests[req.ID] = req

	if s.db != nil {
		requestedAt := req.RequestedAt.UTC().Format(time.RFC3339)
		expiresAt := req.ExpiresAt.UTC().Format(time.RFC3339)
		s.db.Exec(`INSERT INTO permission_requests (id, process, action, old_value, new_value, requested_at, expires_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			req.ID, req.Process, req.Action, req.OldValue, req.NewValue, requestedAt, expiresAt)
	}
}

func (s *MemStore) ResolvePermissionRequest(id string, approved bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.permissionRequests[id]
	if !ok {
		return ErrRequestNotFound
	}
	delete(s.permissionRequests, id)

	if s.db != nil {
		decidedAt := time.Now().UTC().Format(time.RFC3339)
		approvalVal := 0
		if approved {
			approvalVal = 1
		}
		s.db.Exec(`UPDATE permission_requests SET approved=?, decided_at=? WHERE id=?`,
			approvalVal, decidedAt, id)
	}
	return nil
}

func (s *MemStore) PendingPermissionRequests() []domain.PermissionRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.PermissionRequest, 0, len(s.permissionRequests))
	for _, v := range s.permissionRequests {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].RequestedAt.After(result[j].RequestedAt)
	})
	return result
}

func (s *MemStore) GetSetting(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.settings[key]
	if !ok {
		return "", fmt.Errorf("setting %q not found", key)
	}
	return val, nil
}

func (s *MemStore) SetSetting(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.settings[key] = value

	if s.db != nil {
		updatedAt := time.Now().UTC().Format(time.RFC3339)
		s.db.Exec(`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
			key, value, updatedAt)
	}
	return nil
}

func (s *MemStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
