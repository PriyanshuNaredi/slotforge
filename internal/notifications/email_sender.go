package notifications

import "context"

// EmailMessage is a single outbound email.
type EmailMessage struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// EmailSender sends one email. Implementations must be safe for
// concurrent use.
type EmailSender interface {
	SendEmail(ctx context.Context, message EmailMessage) error
}
