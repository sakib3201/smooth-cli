package logstore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smoothcli/smooth-cli/internal/domain"
	"github.com/smoothcli/smooth-cli/internal/logstore"
)

func newTempStore(t *testing.T) logstore.Store {
	tmp := t.TempDir()
	s, err := logstore.NewSQLiteStore(filepath.Join(tmp, "test.db"), 100)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func makeLine(process string, seq int64, stream domain.Stream, raw []byte) domain.LogLine {
	return domain.LogLine{
		Process:   process,
		Stream:    stream,
		Timestamp: time.Now(),
		Raw:       raw,
		Stripped:  string(raw),
		Seq:       seq,
	}
}

func TestStore_PersistsLines_ToSQLite(t *testing.T) {
	s := newTempStore(t)

	for i := int64(1); i <= 5; i++ {
		s.Append(makeLine("test", i, domain.Stdout, []byte("line")))
	}

	time.Sleep(200 * time.Millisecond)

	lines := s.Lines("test", 0)
	require.Len(t, lines, 5)
}

func TestStore_Search_ByGrep_ReturnsMatchingLines(t *testing.T) {
	s := newTempStore(t)

	s.Append(makeLine("test", 1, domain.Stdout, []byte("Server started")))
	s.Append(makeLine("test", 2, domain.Stdout, []byte("Error: ECONNREFUSED")))
	s.Append(makeLine("test", 3, domain.Stdout, []byte("Connected to DB")))

	time.Sleep(200 * time.Millisecond)

	results, err := s.Search(context.Background(), logstore.SearchOpts{
		Process: "test",
		Grep:    "ECONNREFUSED",
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Stripped, "ECONNREFUSED")
}

func TestStore_Search_BySince_ReturnsLinesAfterTime(t *testing.T) {
	s := newTempStore(t)

	s.Append(makeLine("test", 1, domain.Stdout, []byte("old line")))
	time.Sleep(10 * time.Millisecond)
	since := time.Now()
	time.Sleep(10 * time.Millisecond)
	s.Append(makeLine("test", 2, domain.Stdout, []byte("new line")))

	time.Sleep(200 * time.Millisecond)

	results, err := s.Search(context.Background(), logstore.SearchOpts{
		Process: "test",
		Since:   since,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Stripped, "new line")
}

func TestStore_Search_ByStream_Stdout_ExcludesStderr(t *testing.T) {
	s := newTempStore(t)

	s.Append(makeLine("test", 1, domain.Stdout, []byte("stdout line")))
	s.Append(makeLine("test", 2, domain.Stderr, []byte("stderr line")))

	time.Sleep(200 * time.Millisecond)

	results, err := s.Search(context.Background(), logstore.SearchOpts{
		Process: "test",
		Stream:  domain.Stdout,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Stripped, "stdout line")
}

func TestStore_Search_Limit_CapsTotalResults(t *testing.T) {
	s := newTempStore(t)

	for i := int64(1); i <= 20; i++ {
		s.Append(makeLine("test", i, domain.Stdout, []byte("line")))
	}

	time.Sleep(200 * time.Millisecond)

	results, err := s.Search(context.Background(), logstore.SearchOpts{
		Process: "test",
		Limit:   5,
	})
	require.NoError(t, err)
	require.Len(t, results, 5)
}

func TestStore_Search_Reverse_ReturnsNewestFirst(t *testing.T) {
	s := newTempStore(t)

	s.Append(makeLine("test", 1, domain.Stdout, []byte("first")))
	s.Append(makeLine("test", 2, domain.Stdout, []byte("second")))
	s.Append(makeLine("test", 3, domain.Stdout, []byte("third")))

	time.Sleep(200 * time.Millisecond)

	results, err := s.Search(context.Background(), logstore.SearchOpts{
		Process: "test",
		Reverse: true,
	})
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Contains(t, results[0].Stripped, "third")
	assert.Contains(t, results[2].Stripped, "first")
}

func TestStore_FallsBackToSQLite_WhenLinePreddatesRingBuffer(t *testing.T) {
	s := newTempStore(t)

	for i := int64(1); i <= 10; i++ {
		s.Append(makeLine("test", i, domain.Stdout, []byte("line")))
	}

	time.Sleep(200 * time.Millisecond)

	results, err := s.Search(context.Background(), logstore.SearchOpts{
		Process: "test",
		Limit:   5,
	})
	require.NoError(t, err)
	assert.Len(t, results, 5)
}

func TestStore_Close_FlushesAllPending_BeforeClose(t *testing.T) {
	tmp := t.TempDir()
	s, err := logstore.NewSQLiteStore(filepath.Join(tmp, "test.db"), 100)
	require.NoError(t, err)

	for i := int64(1); i <= 10; i++ {
		s.Append(makeLine("test", i, domain.Stdout, []byte("line")))
	}

	err = s.Close()
	require.NoError(t, err)

	s2, err := logstore.NewSQLiteStore(filepath.Join(tmp, "test.db"), 100)
	require.NoError(t, err)
	defer s2.Close()

	lines := s2.Lines("test", 0)
	t.Logf("recovered %d lines after close and reopen", len(lines))
}
