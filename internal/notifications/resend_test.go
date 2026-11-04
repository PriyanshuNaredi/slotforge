package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestResendSender(
	t *testing.T,
	handler http.Handler,
) *ResendSender {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	// Mirror the production endpoint, which includes the /emails path.
	return &ResendSender{
		apiKey:   "test-api-key",
		from:     "SlotForge <test@example.com>",
		endpoint: server.URL + "/emails",
		client:   server.Client(),
	}
}

func TestResendSenderSendsRequest(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotAuth   string
		gotType   string
		gotBody   []byte
	)

	sender := newTestResendSender(t, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			gotType = r.Header.Get("Content-Type")

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read request body: %v", err)
			}
			gotBody = body

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"email-123"}`))
		},
	))

	err := sender.SendEmail(context.Background(), EmailMessage{
		To:      "customer@example.com",
		Subject: "Hello",
		HTML:    "<p>Hello</p>",
		Text:    "Hello",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}

	if gotPath != "/emails" {
		t.Errorf("expected /emails, got %s", gotPath)
	}

	if gotAuth != "Bearer test-api-key" {
		t.Errorf("unexpected Authorization header: %s", gotAuth)
	}

	if !strings.HasPrefix(gotType, "application/json") {
		t.Errorf("unexpected Content-Type header: %s", gotType)
	}

	var payload map[string]any

	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("decode request payload: %v", err)
	}

	if payload["from"] != "SlotForge <test@example.com>" {
		t.Errorf("unexpected from: %v", payload["from"])
	}

	to, ok := payload["to"].([]any)
	if !ok || len(to) != 1 || to[0] != "customer@example.com" {
		t.Errorf("unexpected to: %v", payload["to"])
	}

	if payload["subject"] != "Hello" {
		t.Errorf("unexpected subject: %v", payload["subject"])
	}

	if payload["html"] != "<p>Hello</p>" {
		t.Errorf("unexpected html: %v", payload["html"])
	}

	if payload["text"] != "Hello" {
		t.Errorf("unexpected text: %v", payload["text"])
	}
}

func TestResendSenderUnauthorized(t *testing.T) {
	sender := newTestResendSender(t, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write(
				[]byte(`{"name":"validation_error","message":"Invalid API key"}`),
			)
		},
	))

	err := sender.SendEmail(context.Background(), EmailMessage{
		To:      "customer@example.com",
		Subject: "Hello",
		HTML:    "<p>Hello</p>",
	})

	if err == nil {
		t.Fatal("expected an error for a 401 response")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected status code in error, got: %v", err)
	}

	if strings.Contains(err.Error(), "test-api-key") {
		t.Errorf("error must not contain the API key: %v", err)
	}
}

func TestResendServerNon2xxResponse(t *testing.T) {
	sender := newTestResendSender(t, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"internal error"}`))
		},
	))

	err := sender.SendEmail(context.Background(), EmailMessage{
		To:      "customer@example.com",
		Subject: "Hello",
		HTML:    "<p>Hello</p>",
	})

	if err == nil {
		t.Fatal("expected an error for a 500 response")
	}

	if !strings.Contains(err.Error(), "500") ||
		!strings.Contains(err.Error(), "internal error") {
		t.Errorf("expected status and message in error, got: %v", err)
	}
}

func TestResendSenderNonJSONErrorBody(t *testing.T) {
	sender := newTestResendSender(t, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("Service Unavailable"))
		},
	))

	err := sender.SendEmail(context.Background(), EmailMessage{
		To:      "customer@example.com",
		Subject: "Hello",
		HTML:    "<p>Hello</p>",
	})

	if err == nil {
		t.Fatal("expected an error for a 503 response")
	}

	if !strings.Contains(err.Error(), "Service Unavailable") {
		t.Errorf("expected response body in error, got: %v", err)
	}
}

func TestResendSenderIgnoresMalformedSuccessBody(t *testing.T) {
	sender := newTestResendSender(t, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("not-json"))
		},
	))

	err := sender.SendEmail(context.Background(), EmailMessage{
		To:      "customer@example.com",
		Subject: "Hello",
		HTML:    "<p>Hello</p>",
	})

	if err != nil {
		t.Fatalf("expected success without parsing the body, got %v", err)
	}
}

// errBodyReader fails on every read.
type errBodyReader struct{}

func (errBodyReader) Read(p []byte) (int, error) {
	return 0, errors.New("read error")
}

// okWithBrokenBodyTransport returns a 2xx response whose body cannot be read.
type okWithBrokenBodyTransport struct{}

func (okWithBrokenBodyTransport) RoundTrip(
	r *http.Request,
) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{},
		Body:       io.NopCloser(errBodyReader{}),
		Request:    r,
	}, nil
}

func TestResendSenderSuccessDoesNotReadBody(t *testing.T) {
	// A 2xx response must be treated as success without reading the body, so
	// a body read error after provider acceptance cannot trigger a retry.
	sender := &ResendSender{
		apiKey:   "test-api-key",
		from:     "test@example.com",
		endpoint: "https://api.resend.com/emails",
		client:   &http.Client{Transport: okWithBrokenBodyTransport{}},
	}

	err := sender.SendEmail(context.Background(), EmailMessage{
		To:      "customer@example.com",
		Subject: "Hello",
		HTML:    "<p>Hello</p>",
	})

	if err != nil {
		t.Fatalf(
			"expected success without reading the response body, got %v",
			err,
		)
	}
}

func TestResendSenderNetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {},
	))
	url := server.URL
	transport := server.Client().Transport
	server.Close()

	sender := &ResendSender{
		apiKey:   "test-api-key",
		from:     "test@example.com",
		endpoint: url,
		client: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
		},
	}

	err := sender.SendEmail(context.Background(), EmailMessage{
		To:      "customer@example.com",
		Subject: "Hello",
		HTML:    "<p>Hello</p>",
	})

	if err == nil {
		t.Fatal("expected a network error for a closed server")
	}
}

func TestResendSenderRequestTimeout(t *testing.T) {
	sender := newTestResendSender(t, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
		},
	))
	sender.client = &http.Client{Timeout: 10 * time.Millisecond}

	err := sender.SendEmail(context.Background(), EmailMessage{
		To:      "customer@example.com",
		Subject: "Hello",
		HTML:    "<p>Hello</p>",
	})

	if err == nil {
		t.Fatal("expected a timeout error")
	}
}

func TestResendSenderRequiresRecipient(t *testing.T) {
	sender := newTestResendSender(t, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("no request should be sent without a recipient")
		},
	))

	err := sender.SendEmail(context.Background(), EmailMessage{
		Subject: "Hello",
		HTML:    "<p>Hello</p>",
	})

	if err == nil {
		t.Fatal("expected an error for a missing recipient")
	}
}

func TestTruncateErrorText(t *testing.T) {
	long := strings.Repeat("a", 500)

	got := truncateErrorText(long)

	if len(got) != 303 {
		t.Errorf("expected truncated text of 303 characters, got %d", len(got))
	}

	if got := truncateErrorText("  short  "); got != "short" {
		t.Errorf("expected trimmed text, got %q", got)
	}
}
