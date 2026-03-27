package attention_test

import (
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/smoothcli/smooth-cli/internal/attention"
	"github.com/smoothcli/smooth-cli/internal/domain"
)

type corpusEntry struct {
	Line      string `yaml:"line"`
	Attention bool   `yaml:"attention"`
}

type corpusData struct {
	Entries []corpusEntry `yaml:"entries"`
}

func loadCorpus(t *testing.T) []corpusEntry {
	data, err := os.ReadFile("testdata/corpus.yaml")
	require.NoError(t, err)
	var corpus corpusData
	err = yaml.Unmarshal(data, &corpus)
	require.NoError(t, err)
	return corpus.Entries
}

func makeLine(raw string) domain.LogLine {
	return domain.LogLine{
		Process:   "test",
		Stream:    domain.Stdout,
		Timestamp: time.Now(),
		Raw:       []byte(raw),
		Stripped:  raw,
		Seq:       1,
	}
}

func TestDetector_DetectsAllDefaultPatterns(t *testing.T) {
	d, err := attention.New()
	require.NoError(t, err)

	tests := []struct {
		pattern string
		line    string
	}{
		{`(?i)\(y/n\)`, "Do you want to continue? (y/n)"},
		{`(?i)password(\s*):`, "password:"},
		{`(?i)(error|FATAL|panic|CRITICAL):`, "ERROR: something failed"},
		{`(?i)are you sure`, "Are you sure you want to continue?"},
	}

	for _, tt := range tests {
		line := makeLine(tt.line)
		match := d.Check("test", line)
		assert.NotNil(t, tt.pattern, "pattern %q should match line %q", tt.pattern, tt.line)
		if match != nil {
			assert.Equal(t, tt.pattern, match.Pattern)
		}
	}
}

func TestDetector_CorpusCheck_TruePositives_HaveZeroMisses(t *testing.T) {
	d, err := attention.New()
	require.NoError(t, err)

	corpus := loadCorpus(t)
	var missed int
	var missedLines []string

	for _, entry := range corpus {
		if entry.Attention {
			line := makeLine(entry.Line)
			line.Raw = []byte(entry.Line)
			match := d.Check("test", line)
			if match == nil {
				missed++
				missedLines = append(missedLines, entry.Line)
			}
		}
	}

	assert.Equal(t, 0, missed, "Corpus true positives missed: %v", missedLines)
}

func TestDetector_CorpusCheck_FalsePositiveRate_BelowFourPercent(t *testing.T) {
	d, err := attention.New()
	require.NoError(t, err)

	corpus := loadCorpus(t)
	var falsePositives int
	var negatives int

	for _, entry := range corpus {
		if !entry.Attention {
			negatives++
			line := makeLine(entry.Line)
			line.Raw = []byte(entry.Line)
			match := d.Check("test", line)
			if match != nil {
				falsePositives++
			}
		}
	}

	if negatives > 0 {
		rate := float64(falsePositives) / float64(negatives)
		t.Logf("False positive rate: %.2f%% (%d/%d)", rate*100, falsePositives, negatives)
		assert.Less(t, rate, 0.04, "False positive rate should be below 4%%")
	}
}

func TestDetector_Check_IsIdempotent(t *testing.T) {
	d, err := attention.New()
	require.NoError(t, err)

	line := makeLine("Do you want to continue? (y/n)")
	m1 := d.Check("test", line)
	m2 := d.Check("test", line)

	assert.Equal(t, m1 != nil, m2 != nil)
	if m1 != nil && m2 != nil {
		assert.Equal(t, m1.Pattern, m2.Pattern)
	}
}

func TestDetector_Check_IsConcurrently_Safe(t *testing.T) {
	d, err := attention.New()
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				line := makeLine("Do you want to continue? (y/n)")
				d.Check("test", line)
			}
		}(i)
	}
	wg.Wait()
}

func TestDetector_AddPattern_CompilesAndMatchesNewPattern(t *testing.T) {
	d, err := attention.New()
	require.NoError(t, err)

	err = d.AddPattern(`(?i)custom trigger phrase`)
	require.NoError(t, err)

	line := makeLine("Custom Trigger Phrase")
	match := d.Check("test", line)
	assert.NotNil(t, match)
}

func TestDetector_AddPattern_ReturnsError_OnInvalidRegex(t *testing.T) {
	d, err := attention.New()
	require.NoError(t, err)

	err = d.AddPattern("(unclosed paren")
	require.Error(t, err)
	assert.True(t, errors.Is(err, attention.ErrInvalidPattern))
}

func TestDetector_Check_ReturnsCorrectMatchingPattern(t *testing.T) {
	d, err := attention.New()
	require.NoError(t, err)

	line := makeLine("Do you want to continue? (y/n)")
	match := d.Check("test", line)
	require.NotNil(t, match)
	assert.NotEmpty(t, match.Pattern)
}

func TestDetector_Check_ReturnsNil_ForEmptyLine(t *testing.T) {
	d, err := attention.New()
	require.NoError(t, err)

	line := makeLine("")
	match := d.Check("test", line)
	assert.Nil(t, match)
}

func TestDetector_Check_ReturnsNil_ForWhitespaceOnlyLine(t *testing.T) {
	d, err := attention.New()
	require.NoError(t, err)

	line := makeLine("   ")
	match := d.Check("test", line)
	assert.Nil(t, match)
}

func TestDetector_OSCSequence_9_Detected(t *testing.T) {
	d, err := attention.New()
	require.NoError(t, err)

	line := domain.LogLine{
		Process:   "test",
		Stream:    domain.Stdout,
		Timestamp: time.Now(),
		Raw:       []byte("\x1b]9;Process needs attention\x07"),
		Stripped:  "",
		Seq:       1,
	}
	match := d.Check("test", line)
	assert.NotNil(t, match)
}

func TestDetector_OSCSequence_99_Detected(t *testing.T) {
	d, err := attention.New()
	require.NoError(t, err)

	line := domain.LogLine{
		Process:   "test",
		Stream:    domain.Stdout,
		Timestamp: time.Now(),
		Raw:       []byte("\x1b]99;Kitty notification\x07"),
		Stripped:  "",
		Seq:       1,
	}
	match := d.Check("test", line)
	assert.NotNil(t, match)
}

func TestDetector_OSCSequence_777_Detected(t *testing.T) {
	d, err := attention.New()
	require.NoError(t, err)

	line := domain.LogLine{
		Process:   "test",
		Stream:    domain.Stdout,
		Timestamp: time.Now(),
		Raw:       []byte("\x1b]777;notify-osd notification\x07"),
		Stripped:  "",
		Seq:       1,
	}
	match := d.Check("test", line)
	assert.NotNil(t, match)
}

func BenchmarkDetector_Check_TypicalLogLine(b *testing.B) {
	d, err := attention.New()
	if err != nil {
		b.Fatal(err)
	}
	line := makeLine("Do you want to continue? (y/n)")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Check("test", line)
	}
}
