package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
)

type ServerChan struct {
	SendKey string
	BaseURL string
	Client  *http.Client
}

func NewServerChan() Notifier {
	timeout := config.C.Shop.ServerChan.RequestTimeout
	if timeout <= 0 {
		timeout = 10
	}
	return &ServerChan{
		SendKey: strings.TrimSpace(os.Getenv("SERVERCHAN_SEND_KEY")),
		BaseURL: strings.TrimRight(config.C.Shop.ServerChan.BaseURL, "/"),
		Client:  &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}
}

func (a *ServerChan) Available() bool { return a.SendKey != "" }

func (a *ServerChan) Send(ctx context.Context, title, description string) (string, error) {
	if !a.Available() {
		return "", fmt.Errorf("SERVERCHAN_SEND_KEY is not configured")
	}
	if a.BaseURL == "" {
		return "", fmt.Errorf("ServerChan base URL is not configured")
	}
	endpoint := a.BaseURL + "/" + url.PathEscape(a.SendKey) + ".send"
	values := url.Values{"title": {title}, "desp": {description}, "noip": {"1"}}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(time.Duration(1<<uint(attempt-1)) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
		if err != nil {
			return "", fmt.Errorf("build ServerChan request: %s", redactSendKey(err.Error(), a.SendKey))
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := a.Client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("%s", redactSendKey(err.Error(), a.SendKey))
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
		_ = resp.Body.Close()
		redacted := redactSendKey(string(body), a.SendKey)
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			lastErr = fmt.Errorf("ServerChan returned HTTP %d", resp.StatusCode)
			continue
		}
		var result struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &result); err == nil && result.Code != 0 {
			lastErr = fmt.Errorf("ServerChan returned code %d: %s", result.Code, redactSendKey(result.Message, a.SendKey))
			continue
		}
		return redacted, nil
	}
	return "", fmt.Errorf("ServerChan send failed after retries: %w", lastErr)
}

func redactSendKey(value, key string) string {
	if key == "" {
		return value
	}
	return strings.ReplaceAll(value, key, "[REDACTED]")
}
