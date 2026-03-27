package logstore_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smoothcli/smooth-cli/internal/domain"
	"github.com/smoothcli/smooth-cli/internal/logstore"
)

func makeLogLine(seq int64, raw []byte) domain.LogLine {
	return domain.LogLine{
		Process:   "test",
		Stream:    domain.Stdout,
		Timestamp: time.Now(),
		Raw:       raw,
		Stripped:  string(raw),
		Seq:       seq,
	}
}

func TestRingBuf_StoresLines_UpToCapacity(t *testing.T) {
	buf := logstore.NewRingBuf(10)

	for i := 1; i <= 10; i++ {
		buf.Append(makeLogLine(int64(i), []byte("line")))
	}

	lines := buf.Lines(0)
	require.Len(t, lines, 10)
	assert.Equal(t, int64(1), lines[0].Seq)
	assert.Equal(t, int64(10), lines[9].Seq)
}

func TestRingBuf_EvictsOldestLine_WhenFull(t *testing.T) {
	buf := logstore.NewRingBuf(10)

	for i := 1; i <= 11; i++ {
		buf.Append(makeLogLine(int64(i), []byte("line")))
	}

	lines := buf.Lines(0)
	require.Len(t, lines, 10)
	assert.Equal(t, int64(2), lines[0].Seq)
	assert.Equal(t, int64(11), lines[9].Seq)
}

func TestRingBuf_Returns_CorrectN_Lines(t *testing.T) {
	buf := logstore.NewRingBuf(100)

	for i := 1; i <= 100; i++ {
		buf.Append(makeLogLine(int64(i), []byte("line")))
	}

	lines := buf.Lines(10)
	require.Len(t, lines, 10)
	assert.Equal(t, int64(91), lines[0].Seq)
	assert.Equal(t, int64(100), lines[9].Seq)
}

func TestRingBuf_Returns_AllLines_WhenNIsZero(t *testing.T) {
	buf := logstore.NewRingBuf(5)

	for i := 1; i <= 3; i++ {
		buf.Append(makeLogLine(int64(i), []byte("line")))
	}

	lines := buf.Lines(0)
	require.Len(t, lines, 3)
}

func TestRingBuf_Clear_EmptiesBuffer(t *testing.T) {
	buf := logstore.NewRingBuf(10)

	for i := 1; i <= 5; i++ {
		buf.Append(makeLogLine(int64(i), []byte("line")))
	}
	buf.Clear()

	lines := buf.Lines(0)
	require.Len(t, lines, 0)
	assert.Equal(t, int64(-1), buf.OldestSeq())
}

func TestRingBuf_PreservesANSI_InRawField(t *testing.T) {
	buf := logstore.NewRingBuf(10)
	raw := []byte("\033[32mhello\033[0m")

	buf.Append(makeLogLine(1, raw))

	lines := buf.Lines(1)
	require.Len(t, lines, 1)
	assert.Equal(t, "\033[32mhello\033[0m", string(lines[0].Raw))
	assert.Equal(t, "\033[32mhello\033[0m", lines[0].Stripped)
}

func TestRingBuf_IsConcurrently_Safe(t *testing.T) {
	buf := logstore.NewRingBuf(1000)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				buf.Append(makeLogLine(int64(n*100+j), []byte("line")))
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				buf.Lines(10)
			}
		}()
	}

	wg.Wait()
}
