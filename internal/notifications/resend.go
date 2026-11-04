package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// emailSendTimeout bounds a single provider send attempt. It is shared with
// the worker's batch-safety validation so the two cannot drift apart.
const emailSendTimeout = 15 * time.Second

// ResendSender sends emails through Resend's HTTP API. It keeps all
// provider-specific details out of the delivery worker.
type ResendSender struct {
	apiKey   string
	from     string
	endpoint string
	client   *http.Client
}

func NewResendSender(apiKey string, from string) *ResendSender {
	return &ResendSender{
		apiKey:   apiKey,
		from:     from,
		endpoint: "https://api.resend.com/emails",
		client:   &http.Client{Timeout: emailSendTimeout},
	}
}

type resendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text,omitempty"`
}

func (s *ResendSender) SendEmail(
	ctx context.Context,
	message EmailMessage,
) error {
	if message.To == "" {
		return fmt.Errorf("resend send: recipient is required")
	}

	payload, err := json.Marshal(resendEmailRequest{
		From:    s.from,
		To:      []string{message.To},
		Subject: message.Subject,
		HTML:    message.HTML,
		Text:    message.Text,
	})
	if err != nil {
		return fmt.Errorf("resend encode request: %w", err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		s.endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("resend build request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+s.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("resend request: %w", err)
	}
	defer response.Body.Close()

	// A 2xx response means Resend accepted the email. Return before touching
	// the body so a body read error after successful provider acceptance
	// cannot trigger an unnecessary retry.
	if response.StatusCode >= 200 && response.StatusCode <= 299 {
		return nil
	}

	// Provider error bodies are small; never buffer more than 4 KB.
	body, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	if err != nil {
		return fmt.Errorf("resend read response: %w", err)
	}

	return fmt.Errorf(
		"resend request failed with status %d: %s",
		response.StatusCode,
		truncateErrorText(string(body)),
	)
}

// truncateErrorText keeps provider errors short before they are logged or
// stored in last_error.
func truncateErrorText(text string) string {
	text = strings.TrimSpace(text)

	if len(text) > 300 {
		return text[:300] + "..."
	}

	return text
}
