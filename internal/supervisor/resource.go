package supervisor

import (
	"context"
	"time"

	"github.com/smoothcli/smooth-cli/internal/events"
)

const resourceSampleInterval = 5 * time.Second

func sampleResources(ctx context.Context, p *process, bus events.Bus) {
	ticker := time.NewTicker(resourceSampleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.mu.Lock()
			pid := p.pid
			p.mu.Unlock()

			if pid <= 0 {
				continue
			}

			cpu, mem := readProcessResources(pid)
			p.mu.Lock()
			p.cpuPercent = cpu
			p.memoryMB = mem
			snap := p.snapshot()
			p.mu.Unlock()
			bus.Publish(events.NewEvent(events.KindResourceSample,
				events.ProcessEvent{State: snap}))
		}
	}
}
