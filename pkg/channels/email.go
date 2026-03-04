package channels

import (
	"context"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/zeroclaw-labs/goclaw/pkg/types"
)

type EmailChannel struct {
	smtpHost     string
	smtpPort     int
	username     string
	password     string
	fromAddress  string
	allowedEmails []string
}

func NewEmailChannel(smtpHost string, smtpPort int, username, password, fromAddress string, allowedEmails []string) *EmailChannel {
	return &EmailChannel{
		smtpHost:      smtpHost,
		smtpPort:      smtpPort,
		username:      username,
		password:      password,
		fromAddress:   fromAddress,
		allowedEmails: normalizeAllowedEmails(allowedEmails),
	}
}

func (c *EmailChannel) Name() string {
	return "email"
}

func (c *EmailChannel) Send(ctx context.Context, message *types.SendMessage) error {
	if err := c.validateEmail(message.Recipient); err != nil {
		return fmt.Errorf("invalid recipient email: %w", err)
	}

	subject := "GoClaw Response"
	if message.Subject != nil {
		subject = *message.Subject
	}

	headers := make(map[string]string)
	headers["From"] = c.fromAddress
	headers["To"] = message.Recipient
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=\"utf-8\""

	var msg strings.Builder
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(message.Content)

	auth := smtp.PlainAuth("", c.username, c.password, c.smtpHost)
	addr := fmt.Sprintf("%s:%d", c.smtpHost, c.smtpPort)

	err := smtp.SendMail(addr, auth, c.fromAddress, []string{message.Recipient}, []byte(msg.String()))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (c *EmailChannel) Listen(ctx context.Context, msgChan chan<- types.ChannelMessage) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Email channel is typically poll-based
			// Implement IMAP polling here if needed
		}
	}
}

func (c *EmailChannel) HealthCheck(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", c.smtpHost, c.smtpPort)

	conn, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	if err := conn.Hello("localhost"); err != nil {
		return fmt.Errorf("SMTP HELO failed: %w", err)
	}

	if c.username != "" && c.password != "" {
		if ok, _ := conn.Extension("STARTTLS"); ok {
			if err := conn.StartTLS(nil); err != nil {
				return fmt.Errorf("SMTP STARTTLS failed: %w", err)
			}
		}

		auth := smtp.PlainAuth("", c.username, c.password, c.smtpHost)
		if err := conn.Auth(auth); err != nil {
			return fmt.Errorf("SMTP AUTH failed: %w", err)
		}
	}

	return nil
}

func (c *EmailChannel) StartTyping(ctx context.Context, recipient string) error {
	return nil
}

func (c *EmailChannel) StopTyping(ctx context.Context, recipient string) error {
	return nil
}

func (c *EmailChannel) SupportsDraftUpdates() bool {
	return false
}

func (c *EmailChannel) SendDraft(ctx context.Context, message *types.SendMessage) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *EmailChannel) UpdateDraft(ctx context.Context, recipient, messageID, text string) (string, error) {
	return "", fmt.Errorf("draft updates not supported")
}

func (c *EmailChannel) FinalizeDraft(ctx context.Context, recipient, messageID, text string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *EmailChannel) CancelDraft(ctx context.Context, recipient, messageID string) error {
	return fmt.Errorf("draft updates not supported")
}

func (c *EmailChannel) AddReaction(ctx context.Context, channelID, messageID, emoji string) error {
	return fmt.Errorf("reactions not supported")
}

func (c *EmailChannel) RemoveReaction(ctx context.Context, channelID, messageID, emoji string) error {
	return fmt.Errorf("reactions not supported")
}

func (c *EmailChannel) isEmailAllowed(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))

	if len(c.allowedEmails) == 0 {
		return true
	}

	for _, allowed := range c.allowedEmails {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if allowed == "*" || allowed == email {
			return true
		}

		if strings.HasSuffix(allowed, "@*") {
			domain := strings.TrimSuffix(allowed, "@*")
			if strings.HasSuffix(email, "@"+domain) {
				return true
			}
		}
	}

	return false
}

func (c *EmailChannel) validateEmail(email string) error {
	_, err := mail.ParseAddress(email)
	return err
}

func normalizeAllowedEmails(emails []string) []string {
	result := make([]string, 0, len(emails))
	seen := make(map[string]bool)

	for _, email := range emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			continue
		}

		if _, err := mail.ParseAddress(email); err == nil {
			if !seen[email] {
				seen[email] = true
				result = append(result, email)
			}
		}
	}

	return result
}