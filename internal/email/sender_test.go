package email

import (
	"context"
	"strings"
	"testing"
)

func TestEncodeMessage(t *testing.T) {
	from := "sender@test.com"
	to := []string{"rcpt1@test.com", "rcpt2@test.com"}
	subject := "Test Subject"
	body := "<h1>Hello</h1>"

	msg := encodeMessage(from, to, subject, body)

	if !strings.Contains(msg, "From: sender@test.com") {
		t.Errorf("expected from address, got: %s", msg)
	}
	if !strings.Contains(msg, "To: rcpt1@test.com,rcpt2@test.com") {
		t.Errorf("expected joined recipients, got: %s", msg)
	}
	if !strings.Contains(msg, "Subject: Test Subject") {
		t.Errorf("expected subject, got: %s", msg)
	}
	if !strings.Contains(msg, body) {
		t.Errorf("expected body, got: %s", msg)
	}
	if !strings.Contains(msg, "Content-Type: text/html; charset=\"utf-8\"") {
		t.Errorf("expected content type, got: %s", msg)
	}
}

func TestNewSender(t *testing.T) {
	s := NewSender("creds.json", "token.json", "sender@test.com", []string{"rcpt@test.com"})
	if s == nil {
		t.Fatal("expected non-nil sender")
	}

	gs, ok := s.(*gmailSender)
	if !ok {
		t.Fatal("expected *gmailSender type")
	}

	if gs.credentialsPath != "creds.json" || gs.tokenPath != "token.json" {
		t.Errorf("mismatched paths: %+v", gs)
	}
}

func TestSendHTML_Errors(t *testing.T) {
	// Test without recipients
	s := NewSender("creds.json", "token.json", "sender@test.com", []string{})
	err := s.SendHTML(context.Background(), "Sub", "Body")
	if err == nil || !strings.Contains(err.Error(), "recipients must not be empty") {
		t.Errorf("expected recipients error, got %v", err)
	}

	// Test with missing credentials
	s = NewSender("nonexistent.json", "token.json", "sender@test.com", []string{"a@b.com"})
	err = s.SendHTML(context.Background(), "Sub", "Body")
	if err == nil || !strings.Contains(err.Error(), "unable to read client secret file") {
		t.Errorf("expected file read error, got %v", err)
	}
}
