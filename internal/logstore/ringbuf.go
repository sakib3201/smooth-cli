package logstore

import (
	"sync"

	"github.com/smoothcli/smooth-cli/internal/domain"
)

type RingBuf struct {
	mu       sync.Mutex
	buf      []domain.LogLine
	capacity int
	head     int
	size     int
}

func NewRingBuf(capacity int) *RingBuf {
	return &RingBuf{
		buf:      make([]domain.LogLine, capacity),
		capacity: capacity,
	}
}

func (r *RingBuf) Append(line domain.LogLine) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size < r.capacity {
		r.buf[(r.head+r.size)%r.capacity] = line
		r.size++
	} else {
		r.buf[r.head] = line
		r.head = (r.head + 1) % r.capacity
	}
}

func (r *RingBuf) Lines(n int) []domain.LogLine {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size == 0 {
		return nil
	}
	count := r.size
	if n > 0 && n < count {
		count = n
	}
	out := make([]domain.LogLine, count)
	start := (r.head + r.size - count) % r.capacity
	for i := 0; i < count; i++ {
		out[i] = r.buf[(start+i)%r.capacity]
	}
	return out
}

func (r *RingBuf) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.head = 0
	r.size = 0
}

func (r *RingBuf) OldestSeq() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size == 0 {
		return -1
	}
	return r.buf[r.head].Seq
}
