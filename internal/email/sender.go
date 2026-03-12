package email

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// Sender sends HTML emails via Gmail API.
type Sender interface {
	SendHTML(ctx context.Context, subject, htmlBody string) error
}

type gmailSender struct {
	credentialsPath string
	tokenPath       string
	senderMail      string
	recipients      []string
}

// NewSender creates a new Gmail API based email Sender.
func NewSender(credentialsPath, tokenPath, senderMail string, recipients []string) Sender {
	return &gmailSender{
		credentialsPath: credentialsPath,
		tokenPath:       tokenPath,
		senderMail:      senderMail,
		recipients:      recipients,
	}
}

func (s *gmailSender) SendHTML(ctx context.Context, subject, htmlBody string) error {
	if len(s.recipients) == 0 {
		return errors.New("recipients must not be empty")
	}

	b, err := os.ReadFile(s.credentialsPath)
	if err != nil {
		return fmt.Errorf("unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, gmail.GmailSendScope)
	if err != nil {
		return fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	client, err := s.getClient(ctx, config)
	if err != nil {
		return err
	}

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("unable to retrieve Gmail client: %v", err)
	}

	rawMsg := encodeMessage(s.senderMail, s.recipients, subject, htmlBody)

	message := gmail.Message{
		Raw: base64.URLEncoding.EncodeToString([]byte(rawMsg)),
	}

	_, err = srv.Users.Messages.Send("me", &message).Do()
	if err != nil {
		return fmt.Errorf("failed to send Gmail API message: %v", err)
	}

	return nil
}

func encodeMessage(from string, to []string, subject, htmlBody string) string {
	encodedSubject := mime.QEncoding.Encode("utf-8", subject)
	return fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"utf-8\"\r\n\r\n%s",
		from,
		strings.Join(to, ","),
		encodedSubject,
		htmlBody,
	)
}

// getClient retrieves a token, saves the token, then returns the generated client.
func (s *gmailSender) getClient(ctx context.Context, config *oauth2.Config) (*http.Client, error) {
	tok, err := s.tokenFromFile(s.tokenPath)
	if err != nil {
		tok, err = s.getTokenFromWeb(ctx, config)
		if err != nil {
			return nil, err
		}
		s.saveToken(s.tokenPath, tok)
	}
	return config.Client(ctx, tok), nil
}

// getTokenFromWeb requests a token from the web, then returns the retrieved token.
func (s *gmailSender) getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("--- GMAIL API AUTHORIZATION REQUIRED ---\n")
	fmt.Printf("Go to the following link in your browser: \n%v\n", authURL)
	fmt.Printf("After authorizing the app, enter the verification code here: ")

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %v", err)
	}
	return tok, nil
}

// tokenFromFile retrieves a token from a local file.
func (s *gmailSender) tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(filepath.Clean(file))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// saveToken saves a token to a file path.
func (s *gmailSender) saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(filepath.Clean(path), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		log.Fatalf("unable to cache oauth token: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := json.NewEncoder(f).Encode(token); err != nil { //nolint:gosec // G117: intentionally saving oauth token to file
		log.Fatalf("unable to encode oauth token: %v", err)
	}
}
