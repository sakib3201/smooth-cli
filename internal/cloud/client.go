package cloud

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/smoothcli/smooth-cli/internal/domain"
)

var (
	ErrUnauthorized = errors.New("cloud token rejected (401)")
	ErrPlanRequired = errors.New("plan required (403)")
	ErrRateLimit    = errors.New("rate limited (429)")
	ErrNoToken      = errors.New("no cloud token configured")
)

type Client interface {
	SyncLogs(ctx context.Context, lines []domain.LogLine) error
}

type client struct {
	httpClient *http.Client
	token      string
	baseURL    string
}

type credentials struct {
	Token string `json:"token"`
}

func New(baseURL string) (Client, error) {
	token := os.Getenv("SMOOTH_TOKEN")
	if token == "" {
		token = loadCredentials()
	}
	return &client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		token:      token,
		baseURL:    baseURL,
	}, nil
}

func loadCredentials() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".smooth", "credentials"))
	if err != nil {
		return ""
	}
	var creds credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return ""
	}
	return creds.Token
}

func (c *client) SyncLogs(ctx context.Context, lines []domain.LogLine) error {
	if c.token == "" {
		return ErrNoToken
	}
	payload, err := json.Marshal(lines)
	if err != nil {
		return fmt.Errorf("cloud: marshal: %w", err)
	}
	var body bytes.Buffer
	gzWriter := gzip.NewWriter(&body)
	gzWriter.Write(payload)
	gzWriter.Close()

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Second * time.Duration(attempt))
		}
		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/logs", &body)
		if err != nil {
			lastErr = fmt.Errorf("cloud: new request: %w", err)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("cloud: do: %w", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("cloud: server error %d", resp.StatusCode)
			continue
		}

		switch resp.StatusCode {
		case 200, 201:
			return nil
		case 401:
			return ErrUnauthorized
		case 403:
			return ErrPlanRequired
		case 429:
			lastErr = ErrRateLimit
		default:
			bodyBytes, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("cloud: unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
		}
	}
	return lastErr
}
