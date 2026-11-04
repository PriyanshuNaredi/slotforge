package notifications

import (
	"html"
	"strings"
	"testing"
)

func TestRenderEmailContent(t *testing.T) {
	tests := []struct {
		name         string
		notification Notification
		wantSubject  string
	}{
		{
			name:         "booking_confirmed",
			notification: Notification{Type: "booking_confirmed", Title: "Booking confirmed", Message: "Your booking has been confirmed."},
			wantSubject:  "Your booking is confirmed",
		},
		{
			name:         "new_booking",
			notification: Notification{Type: "new_booking", Title: "New booking received", Message: "A customer has booked one of your available slots."},
			wantSubject:  "New booking received",
		},
		{
			name:         "booking_cancelled",
			notification: Notification{Type: "booking_cancelled", Title: "Booking cancelled", Message: "Your booking has been cancelled."},
			wantSubject:  "Your booking was cancelled",
		},
		{
			name:         "customer_cancelled",
			notification: Notification{Type: "customer_cancelled", Title: "Customer cancelled booking", Message: "A customer has cancelled their booking."},
			wantSubject:  "Booking cancellation confirmed",
		},
		{
			name:         "booking_cancelled_by_provider",
			notification: Notification{Type: "booking_cancelled_by_provider", Title: "Booking cancelled by provider", Message: "The provider cancelled this booking."},
			wantSubject:  "Your booking was cancelled",
		},
		{
			name:         "booking_completed",
			notification: Notification{Type: "booking_completed", Title: "Booking completed", Message: "Thanks for your visit."},
			wantSubject:  "Your booking is complete",
		},
		{
			name:         "booking_no_show",
			notification: Notification{Type: "booking_no_show", Title: "Booking marked as no-show", Message: "You missed your appointment."},
			wantSubject:  "Your booking was marked as a no-show",
		},
		{
			name:         "unknown type falls back to notification title",
			notification: Notification{Type: "mystery_event", Title: "Something happened", Message: "Details inside."},
			wantSubject:  "Something happened",
		},
		{
			name:         "unknown type without title",
			notification: Notification{Type: "mystery_event", Message: "Details inside."},
			wantSubject:  "SlotForge notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := renderEmailContent(&tt.notification)

			if content.Subject != tt.wantSubject {
				t.Errorf("expected subject %q, got %q",
					tt.wantSubject, content.Subject)
			}

			if content.HTML == "" {
				t.Error("expected non-empty HTML body")
			}

			if content.Text == "" {
				t.Error("expected non-empty text body")
			}

			// The notification title and message must appear in both bodies.
			title := tt.notification.Title
			if title == "" {
				title = "Notification"
			}

			if !strings.Contains(content.Text, title) ||
				!strings.Contains(content.Text, tt.notification.Message) {
				t.Errorf("text body missing title or message: %q", content.Text)
			}
		})
	}
}

func TestRenderEmailContentEscapesUserContent(t *testing.T) {
	notification := &Notification{
		Type:    "booking_confirmed",
		Title:   `Booking <script>alert("x")</script>`,
		Message: `Hi <b>there</b> & welcome`,
	}

	content := renderEmailContent(notification)

	if strings.Contains(content.HTML, "<script>") {
		t.Errorf("HTML body must not contain unescaped script tags: %s",
			content.HTML)
	}

	if !strings.Contains(content.HTML, html.EscapeString(notification.Title)) {
		t.Errorf("HTML body must contain the escaped title: %s", content.HTML)
	}

	if !strings.Contains(content.HTML, "Hi &lt;b&gt;there&lt;/b&gt; &amp; welcome") {
		t.Errorf("HTML body must contain the escaped message: %s", content.HTML)
	}
}

func TestRenderEmailContentSanitizesSubjectLineBreaks(t *testing.T) {
	notification := &Notification{
		Type:  "mystery_event",
		Title: "Booking\r\nBcc: attacker@example.com\n",
	}

	content := renderEmailContent(notification)

	if strings.ContainsAny(content.Subject, "\r\n") {
		t.Errorf("subject must not contain CR or LF: %q", content.Subject)
	}

	if content.Subject != "Booking Bcc: attacker@example.com" {
		t.Errorf("unexpected sanitized subject: %q", content.Subject)
	}
}

func TestRenderEmailContentWhitespaceOnlySubjectFallsBack(t *testing.T) {
	notification := &Notification{
		Type:  "mystery_event",
		Title: " \r\n\t ",
	}

	content := renderEmailContent(notification)

	if content.Subject != "SlotForge notification" {
		t.Errorf("expected fallback subject, got %q", content.Subject)
	}
}
