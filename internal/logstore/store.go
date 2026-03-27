package logstore

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/smoothcli/smooth-cli/internal/domain"
	"github.com/smoothcli/smooth-cli/internal/store"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07`)

type Store interface {
	Append(line domain.LogLine)
	Lines(process string, n int) []domain.LogLine
	Search(ctx context.Context, opts SearchOpts) ([]domain.LogLine, error)
	Clear(process string)
	Close() error
}

type SearchOpts struct {
	Process string
	Grep    string
	Stream  domain.Stream
	Since   time.Time
	Until   time.Time
	Limit   int
	Reverse bool
}

type sqliteStore struct {
	mu      sync.RWMutex
	bufs    map[string]*RingBuf
	db      *sql.DB
	flushCh chan domain.LogLine
	done    chan struct{}
}

func NewSQLiteStore(dbPath string, logBufferLines int) (Store, error) {
	db, err := store.OpenDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("logstore: open db: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS log_lines (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			process   TEXT NOT NULL,
			stream    TEXT NOT NULL,
			timestamp TEXT NOT NULL,
			raw       BLOB NOT NULL,
			stripped  TEXT NOT NULL,
			seq       INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_log_process_timestamp ON log_lines(process, timestamp);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("logstore: create table: %w", err)
	}

	s := &sqliteStore{
		bufs:    make(map[string]*RingBuf),
		db:      db,
		flushCh: make(chan domain.LogLine, 1000),
		done:    make(chan struct{}),
	}

	for name := range getProcessNames(db) {
		s.bufs[name] = NewRingBuf(logBufferLines)
	}

	go s.flushLoop()

	return s, nil
}

func getProcessNames(db *sql.DB) map[string]struct{} {
	names := make(map[string]struct{})
	rows, err := db.Query("SELECT DISTINCT process FROM log_lines")
	if err != nil {
		return names
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			names[name] = struct{}{}
		}
	}
	return names
}

func (s *sqliteStore) flushLoop() {
	batch := make([]domain.LogLine, 0, 100)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			for line := range s.flushCh {
				batch = append(batch, line)
			}
			s.flushBatch(batch)
			return
		case line := <-s.flushCh:
			batch = append(batch, line)
			if len(batch) >= 100 {
				s.flushBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				s.flushBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (s *sqliteStore) flushBatch(batch []domain.LogLine) {
	if len(batch) == 0 {
		return
	}

	s.db.Exec("BEGIN TRANSACTION")
	for _, line := range batch {
		timestamp := line.Timestamp.UTC().Format(time.RFC3339Nano)
		stream := string(line.Stream)
		_, err := s.db.Exec(
			`INSERT INTO log_lines (process, stream, timestamp, raw, stripped, seq) VALUES (?, ?, ?, ?, ?, ?)`,
			line.Process, stream, timestamp, line.Raw, line.Stripped, line.Seq,
		)
		if err != nil {
			fmt.Printf("logstore: flush batch error: %v\n", err)
		}
	}
	s.db.Exec("COMMIT")
}

func (s *sqliteStore) Append(line domain.LogLine) {
	s.mu.Lock()
	buf, ok := s.bufs[line.Process]
	if !ok {
		buf = NewRingBuf(10000)
		s.bufs[line.Process] = buf
	}
	s.mu.Unlock()

	stripped := stripANSI(string(line.Raw))
	line.Stripped = stripped

	buf.Append(line)

	select {
	case s.flushCh <- line:
	default:
	}
}

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func (s *sqliteStore) Lines(process string, n int) []domain.LogLine {
	s.mu.RLock()
	buf, ok := s.bufs[process]
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	return buf.Lines(n)
}

func (s *sqliteStore) Search(ctx context.Context, opts SearchOpts) ([]domain.LogLine, error) {
	s.mu.RLock()
	buf, ok := s.bufs[opts.Process]
	oldestSeq := int64(-1)
	if ok {
		oldestSeq = buf.OldestSeq()
	}
	s.mu.RUnlock()

	ringResults := false
	if opts.Since.IsZero() || (oldestSeq >= 0 && opts.Since.Before(time.Now().Add(-24*time.Hour))) {
		ringResults = true
	}

	var results []domain.LogLine

	if ringResults {
		s.mu.RLock()
		if buf != nil {
			lines := buf.Lines(0)
			results = filterLines(lines, opts)
		}
		s.mu.RUnlock()
	} else {
		query := `SELECT process, stream, timestamp, raw, stripped, seq FROM log_lines WHERE process=?`
		args := []interface{}{opts.Process}
		argIdx := 1

		if opts.Grep != "" {
			query += fmt.Sprintf(" AND stripped LIKE $%d", argIdx+1)
			args = append(args, "%"+opts.Grep+"%")
			argIdx++
		}
		if opts.Stream != "" {
			query += fmt.Sprintf(" AND stream=$%d", argIdx+1)
			args = append(args, string(opts.Stream))
			argIdx++
		}
		if !opts.Since.IsZero() {
			query += fmt.Sprintf(" AND timestamp>=$%d", argIdx+1)
			args = append(args, opts.Since.UTC().Format(time.RFC3339Nano))
			argIdx++
		}
		if !opts.Until.IsZero() {
			query += fmt.Sprintf(" AND timestamp<=$%d", argIdx+1)
			args = append(args, opts.Until.UTC().Format(time.RFC3339Nano))
			argIdx++
		}

		if opts.Reverse {
			query += " ORDER BY id DESC"
		} else {
			query += " ORDER BY id ASC"
		}

		if opts.Limit > 0 {
			query += fmt.Sprintf(" LIMIT %d", opts.Limit)
		}

		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("logstore: search: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var line domain.LogLine
			var stream, timestamp, stripped string
			var raw []byte
			if err := rows.Scan(&line.Process, &stream, &timestamp, &raw, &stripped, &line.Seq); err != nil {
				continue
			}
			line.Stream = domain.Stream(stream)
			line.Timestamp, _ = time.Parse(time.RFC3339Nano, timestamp)
			line.Raw = raw
			line.Stripped = stripped
			results = append(results, line)
		}
	}

	return results, nil
}

func filterLines(lines []domain.LogLine, opts SearchOpts) []domain.LogLine {
	var result []domain.LogLine
	for _, line := range lines {
		if opts.Stream != "" && line.Stream != opts.Stream {
			continue
		}
		if !opts.Since.IsZero() && line.Timestamp.Before(opts.Since) {
			continue
		}
		if !opts.Until.IsZero() && line.Timestamp.After(opts.Until) {
			continue
		}
		if opts.Grep != "" && !strings.Contains(strings.ToLower(line.Stripped), strings.ToLower(opts.Grep)) {
			continue
		}
		result = append(result, line)
	}

	if opts.Reverse {
		for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
		}
	}

	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}

	return result
}

func (s *sqliteStore) Clear(process string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if buf, ok := s.bufs[process]; ok {
		buf.Clear()
	}
}

func (s *sqliteStore) Close() error {
	close(s.done)
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
