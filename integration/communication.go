package integration

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"omniChannelservice/tomlloader"
	"omniChannelservice/util"
)

type Communicator struct {
	httpClient *http.Client
	cfg        tomlloader.CommunicationConfig
	log        *util.Logger
}

func NewCommunicator(cfg tomlloader.CommunicationConfig, log *util.Logger) *Communicator {
	return &Communicator{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		cfg:        cfg,
		log:        log,
	}
}

func (c *Communicator) SendSMS(ctx context.Context, payload any) error {
	return c.send(ctx, "sms", c.cfg.SMS, payload)
}

func (c *Communicator) SendWhatsApp(ctx context.Context, payload any) error {
	return c.send(ctx, "whatsapp", c.cfg.WhatsApp, payload)
}

func (c *Communicator) SendEmail(ctx context.Context, payload any) error {
	return c.send(ctx, "email", c.cfg.Email, payload)
}

func (c *Communicator) send(ctx context.Context, name string, cfg tomlloader.ServiceConfig, payload any) error {
	if !cfg.Enabled {
		return fmt.Errorf("%s service disabled", name)
	}

	switch name {
	case "sms":
		fmt.Printf("SMS payload: %+v\n", payload)
		body, err := c.buildInfobipSMSBody(cfg, payload)
		if err != nil {
			return fmt.Errorf("marshal %s payload: %w", name, err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.ServiceURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create %s request: %w", name, err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "App "+cfg.APIKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("%s request failed: %w", name, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("%s returned status %d: %s", name, resp.StatusCode, string(respBody))
		}

		c.log.Info("%s request sent to %s", name, cfg.ServiceURL)
		return nil
	case "email":
		return c.sendEmail(ctx, cfg, payload)
	default:
		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal %s payload: %w", name, err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.ServiceURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create %s request: %w", name, err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "App "+cfg.APIKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("%s request failed: %w", name, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("%s returned status %d: %s", name, resp.StatusCode, string(respBody))
		}

		c.log.Info("%s request sent to %s", name, cfg.ServiceURL)
		return nil
	}
}

func (c *Communicator) sendEmail(ctx context.Context, cfg tomlloader.ServiceConfig, payload any) error {
	if cfg.SMTPHost == "" || cfg.SMTPPort == 0 || cfg.SMTPUser == "" || cfg.SMTPPass == "" || cfg.FromEmail == "" {
		return fmt.Errorf("email SMTP configuration is incomplete")
	}

	data, ok := payload.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid email payload")
	}

	rootSubject, _ := data["subject"].(string)
	rootBody, _ := data["body"].(string)

	if messages, ok := data["messages"].([]any); ok && len(messages) > 0 {
		var firstErr error
		for idx, item := range messages {
			msg, ok := item.(map[string]any)
			if !ok {
				if firstErr == nil {
					firstErr = fmt.Errorf("invalid message at index %d", idx)
				}
				continue
			}

			to, _ := msg["to"].(string)
			subject, _ := msg["subject"].(string)
			bodyText, _ := msg["body"].(string)

			if subject == "" {
				subject = rootSubject
			}
			if bodyText == "" {
				bodyText = rootBody
			}

			if to == "" {
				if firstErr == nil {
					firstErr = fmt.Errorf("email message at index %d missing to", idx)
				}
				continue
			}
			if bodyText == "" {
				if firstErr == nil {
					firstErr = fmt.Errorf("email message at index %d missing body", idx)
				}
				continue
			}

			if err := c.sendSingleEmail(cfg, to, subject, bodyText); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
		}

		return firstErr
	}

	to, _ := data["to"].(string)
	subject, _ := data["subject"].(string)
	bodyText, _ := data["body"].(string)

	if to == "" {
		return fmt.Errorf("email payload must include to")
	}
	if bodyText == "" {
		return fmt.Errorf("email payload must include body")
	}

	return c.sendSingleEmail(cfg, to, subject, bodyText)
}

func (c *Communicator) sendSingleEmail(cfg tomlloader.ServiceConfig, to, subject, bodyText string) error {
	headers := map[string]string{
		"From":         cfg.FromEmail,
		"To":           to,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=\"utf-8\"",
	}

	var msg strings.Builder
	for k, v := range headers {
		if v != "" {
			msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
		}
	}
	msg.WriteString("\r\n")
	msg.WriteString(bodyText)

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)

	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	defer client.Close()

	if err := client.StartTLS(&tls.Config{ServerName: cfg.SMTPHost}); err != nil {
		return fmt.Errorf("starttls failed: %w", err)
	}

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth failed: %w", err)
	}

	if err := client.Mail(cfg.FromEmail); err != nil {
		return fmt.Errorf("smtp mail from failed: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to failed: %w", err)
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data failed: %w", err)
	}
	defer wc.Close()

	if _, err := wc.Write([]byte(msg.String())); err != nil {
		return fmt.Errorf("smtp write failed: %w", err)
	}

	return nil
}

func (c *Communicator) buildInfobipSMSBody(cfg tomlloader.ServiceConfig, payload any) ([]byte, error) {
	if cfg.SMSSender == "" {
		return nil, fmt.Errorf("sms sender is not configured")
	}

	data, ok := payload.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid sms payload")
	}

	if messages, ok := data["messages"].([]any); ok && len(messages) > 0 {
		infobipMessages := make([]map[string]any, 0, len(messages))
		for _, item := range messages {
			msg, ok := item.(map[string]any)
			if !ok {
				continue
			}

			text, _ := msg["message"].(string)
			to, _ := msg["to"].(string)
			if to == "" && text == "" {
				c.log.Warn("skipping sms message because recipient and message are both empty")
				continue
			}
			if to == "" {
				c.log.Warn("skipping sms message because recipient is empty for message: %q", text)
				continue
			}
			if text == "" {
				c.log.Warn("skipping sms message because message is empty for recipient: %q", to)
				continue
			}

			infobipMessages = append(infobipMessages, map[string]any{
				"from": cfg.SMSSender,
				"destinations": []map[string]any{{"to": to}},
				"content": map[string]any{"text": text},
			})
		}

		smsBody := map[string]any{
			"messages": infobipMessages,
			"flash":    false,
		}

		return json.Marshal(smsBody)
	}

	text, _ := data["message"].(string)
	to, _ := data["to"].(string)

	if to == "" || text == "" {
		return nil, fmt.Errorf("sms payload must include to and message")
	}

	smsBody := map[string]any{
		"messages": []map[string]any{
			{
				"from": cfg.SMSSender,
				"destinations": []map[string]any{{"to": to}},
				"content": map[string]any{"text": text},
			},
		},
		"flash": false,
	}

	return json.Marshal(smsBody)
}
