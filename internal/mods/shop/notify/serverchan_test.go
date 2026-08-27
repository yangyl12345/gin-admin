package notify

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestServerChanSend(t *testing.T) {
	var gotPath, gotTitle, gotDescription string
	client := &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		gotTitle = values.Get("title")
		gotDescription = values.Get("desp")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"code":0,"message":"ok"}`)),
			Request:    r,
		}, nil
	})}

	notifier := &ServerChan{SendKey: "test-key", BaseURL: "https://serverchan.test", Client: client}
	response, err := notifier.Send(context.Background(), "标题", "内容")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotPath != "/test-key.send" || gotTitle != "标题" || gotDescription != "内容" {
		t.Fatalf("unexpected request: path=%s title=%s description=%s", gotPath, gotTitle, gotDescription)
	}
	if strings.Contains(response, "test-key") {
		t.Fatal("provider response leaked SendKey")
	}
}

func TestServerChanUnavailable(t *testing.T) {
	notifier := &ServerChan{Client: &http.Client{Timeout: time.Second}}
	if notifier.Available() {
		t.Fatal("empty SendKey should be unavailable")
	}
	if _, err := notifier.Send(context.Background(), "title", "body"); err == nil {
		t.Fatal("Send() should fail without SendKey")
	}
}

func TestServerChanRedactsSendKeyFromTransportError(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("failed request to " + r.URL.String())
	})}
	notifier := &ServerChan{SendKey: "sensitive-key", BaseURL: "https://serverchan.test", Client: client}
	_, err := notifier.Send(context.Background(), "title", "body")
	if err == nil {
		t.Fatal("Send() should fail")
	}
	if strings.Contains(err.Error(), "sensitive-key") {
		t.Fatalf("transport error leaked SendKey: %v", err)
	}
}
