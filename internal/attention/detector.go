package attention

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/smoothcli/smooth-cli/internal/domain"
)

var ErrInvalidPattern = errors.New("invalid regex pattern")

var DefaultPatterns = []string{
	`(?i)\(y/n\)`,
	`(?i)\(Y/n\)`,
	`(?i)\[yes/no\]`,
	`(?i)press (enter|any key)`,
	`(?i)do you want to (continue|proceed|overwrite|delete|remove)`,
	`(?i)password(\s*):`,
	`(?i)are you sure`,
	`(?i)waiting for (input|approval|confirmation|user)`,
	`(?i)(permission denied|access denied|operation not permitted)`,
	`(?i)(error|FATAL|panic|CRITICAL):`,
	`(?i)^fatal error`,
	`(?i)segmentation fault`,
	`(?i)(out of memory|OOM killed)`,
	`\x1b\]9;`,
	`\x1b\]99;`,
	`\x1b\]777;`,
	`(?i)overwrite [\w\./]+ \?`,
	`(?i)enter (your )?(api key|token|secret|passphrase)`,
}

type PatternMatch struct {
	Pattern string
	Context string
}

type Detector interface {
	Check(process string, line domain.LogLine) *PatternMatch
	AddPattern(pattern string) error
	Patterns() []string
}

type detector struct {
	mu       sync.RWMutex
	compiled []*regexp.Regexp
	patterns []string
}

func New() (Detector, error) {
	d := &detector{}
	for _, p := range DefaultPatterns {
		if err := d.AddPattern(p); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func NewWithPatterns(extra []string) (Detector, error) {
	d := &detector{}
	for _, p := range DefaultPatterns {
		if err := d.AddPattern(p); err != nil {
			return nil, err
		}
	}
	for _, p := range extra {
		if err := d.AddPattern(p); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (d *detector) AddPattern(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidPattern, err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.patterns = append(d.patterns, pattern)
	d.compiled = append(d.compiled, re)
	return nil
}

func (d *detector) Patterns() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]string, len(d.patterns))
	copy(out, d.patterns)
	return out
}

func (d *detector) Check(_ string, line domain.LogLine) *PatternMatch {
	text := line.Stripped
	if strings.TrimSpace(text) == "" {
		raw := string(line.Raw)
		if strings.TrimSpace(raw) == "" {
			return nil
		}
		text = raw
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	for i, re := range d.compiled {
		if re.MatchString(text) {
			return &PatternMatch{Pattern: d.patterns[i], Context: line.Stripped}
		}
	}
	return nil
}
