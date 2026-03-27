package permission

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/smoothcli/smooth-cli/internal/config"
	"github.com/smoothcli/smooth-cli/internal/domain"
	"github.com/smoothcli/smooth-cli/internal/events"
	"github.com/smoothcli/smooth-cli/internal/store"
)

var (
	ErrTimeout         = errors.New("permission request timed out")
	ErrRequestNotFound = errors.New("permission request not found")
)

type DangerLevel int

const (
	DangerNone     DangerLevel = 0
	DangerLow      DangerLevel = 1
	DangerMedium   DangerLevel = 2
	DangerHigh     DangerLevel = 3
	DangerCritical DangerLevel = 4
)

type pendingRequest struct {
	decisionCh chan bool
	expiresAt  time.Time
}

type Gate interface {
	Evaluate(ctx context.Context, old, new *config.SmoothConfig) error
	Record(id string, approved bool) error
	IsApproved(ctx context.Context, id string) (bool, error)
}

type gate struct {
	mu       sync.Mutex
	requests map[string]*pendingRequest
	bus      events.Bus
	store    store.Store
}

func New(bus events.Bus, store store.Store) Gate {
	return &gate{
		requests: make(map[string]*pendingRequest),
		bus:      bus,
		store:    store,
	}
}

func (g *gate) Evaluate(ctx context.Context, old, new *config.SmoothConfig) error {
	if old == nil || new == nil {
		return nil
	}

	diff := config.Diff(old, new)

	for _, name := range diff.Added {
		level := DangerCritical // used for added processes
		_ = level
		req := g.createRequest(name, "new_process", "", new.Processes[name].Command)
		g.bus.Publish(events.NewEvent(events.KindPermissionReq,
			events.ConfigChangedEvent{
				Added: []string{name},
			}))
		g.store.AddPermissionRequest(req)
	}

	for _, mod := range diff.Modified {
		level := DangerNone
		switch mod.Field {
		case "command":
			level = DangerHigh
		case "cwd":
			level = DangerMedium
		}

		if level >= DangerLow {
			req := g.createRequest(mod.Name, mod.Field+"_changed", mod.Old, mod.New)
			g.bus.Publish(events.NewEvent(events.KindPermissionReq,
				events.ConfigChangedEvent{
					Modified: []events.ConfigFieldDiff{
						{Process: mod.Name, Field: mod.Field, OldValue: mod.Old, NewValue: mod.New},
					},
				}))
			g.store.AddPermissionRequest(req)
		}
	}

	return nil
}

func (g *gate) createRequest(process, action, oldValue, newValue string) domain.PermissionRequest {
	id := uuid.New().String()
	expiresAt := time.Now().Add(5 * time.Minute)

	pr := domain.PermissionRequest{
		ID:          id,
		Process:     process,
		Action:      action,
		OldValue:    oldValue,
		NewValue:    newValue,
		RequestedAt: time.Now(),
		ExpiresAt:   expiresAt,
	}

	decisionCh := make(chan bool, 1)

	g.mu.Lock()
	g.requests[id] = &pendingRequest{
		decisionCh: decisionCh,
		expiresAt:  expiresAt,
	}
	g.mu.Unlock()

	go g.autoDeny(id, expiresAt)

	return pr
}

func (g *gate) autoDeny(id string, expiresAt time.Time) {
	select {
	case <-time.After(time.Until(expiresAt)):
		g.Record(id, false)
	}
}

func (g *gate) Record(id string, approved bool) error {
	g.mu.Lock()
	pr, ok := g.requests[id]
	if !ok {
		g.mu.Unlock()
		return ErrRequestNotFound
	}
	delete(g.requests, id)
	g.mu.Unlock()

	pr.decisionCh <- approved
	close(pr.decisionCh)

	g.store.ResolvePermissionRequest(id, approved)

	g.bus.Publish(events.NewEvent(events.KindPermissionDec,
		domain.PermissionDecision{
			RequestID: id,
			Approved:  approved,
			DecidedAt: time.Now(),
		}))

	return nil
}

func (g *gate) IsApproved(ctx context.Context, id string) (bool, error) {
	g.mu.Lock()
	pr, ok := g.requests[id]
	g.mu.Unlock()
	if !ok {
		return false, ErrRequestNotFound
	}

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case approved := <-pr.decisionCh:
		return approved, nil
	}
}
