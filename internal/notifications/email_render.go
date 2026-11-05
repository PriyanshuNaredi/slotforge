package notifications

import (
	"fmt"
	"html"
	"strings"
)

type emailContent struct {
	Subject string
	HTML    string
	Text    string
}

// emailSubjects maps notification types to email subjects. Types without an
// entry fall back to the notification title, so an unknown type still
// produces a safe, generic email.
var emailSubjects = map[string]string{
	"booking_confirmed":             "Your booking is confirmed",
	"new_booking":                   "New booking received",
	"booking_cancelled":             "Your booking was cancelled",
	"customer_cancelled":            "Booking cancellation confirmed",
	"booking_cancelled_by_provider": "Your booking was cancelled",
	"booking_completed":             "Your booking is complete",
	"booking_no_show":               "Your booking was marked as a no-show",
	"booking_rescheduled":           "Your appointment was rescheduled",
	"customer_rescheduled":          "Booking reschedule confirmed",
}

const emailHTMLLayout = `<!DOCTYPE html>
<html>
  <body style="margin:0;padding:24px;background-color:#f4f5f7;font-family:Arial,Helvetica,sans-serif;color:#1f2933;">
    <div style="max-width:560px;margin:0 auto;background-color:#ffffff;border-radius:8px;padding:32px;">
      <h1 style="margin:0 0 16px;font-size:20px;">%s</h1>
      <p style="margin:0 0 24px;font-size:15px;line-height:1.6;">%s</p>
      <p style="margin:0;font-size:13px;color:#7b8794;">
        You are receiving this email because you have a SlotForge account.
      </p>
    </div>
  </body>
</html>`

// sanitizeEmailSubject collapses whitespace (including CR and LF) so a
// notification title cannot inject email headers into the subject line.
func sanitizeEmailSubject(subject string) string {
	return strings.Join(strings.Fields(subject), " ")
}

// renderEmailContent builds the email subject, HTML body, and plain-text body
// for a notification. The notification title and message are the source of
// truth for the body and are HTML-escaped before insertion.
func renderEmailContent(notification *Notification) emailContent {
	subject, ok := emailSubjects[notification.Type]
	if !ok || subject == "" {
		subject = notification.Title
	}

	// Sanitize fallback titles too; fall back if nothing is left.
	subject = sanitizeEmailSubject(subject)
	if subject == "" {
		subject = "SlotForge notification"
	}

	title := notification.Title
	if title == "" {
		title = "Notification"
	}

	htmlBody := fmt.Sprintf(
		emailHTMLLayout,
		html.EscapeString(title),
		html.EscapeString(notification.Message),
	)

	textBody := fmt.Sprintf(
		"%s\n\n%s\n\nYou are receiving this email because you have a SlotForge account.",
		title,
		notification.Message,
	)

	return emailContent{
		Subject: subject,
		HTML:    htmlBody,
		Text:    textBody,
	}
}
