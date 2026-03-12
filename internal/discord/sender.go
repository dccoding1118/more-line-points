package discord

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bwmarrin/discordgo"
)

// Sender sends messages via Discord.
type Sender interface {
	SendMessage(ctx context.Context, text string) error
	Close() error
}

type botSender struct {
	session         *discordgo.Session
	notifyChannelID string
}

// NewSender creates a new Discord Sender initializing a discordgo Session.
// If apiEndpoint is provided, it globally overrides the discord API endpoint.
func NewSender(botToken, notifyChannelID, apiEndpoint string) (Sender, error) {
	if apiEndpoint != "" {
		// discordgo expects a trailing slash for EndpointAPI
		if len(apiEndpoint) > 0 && apiEndpoint[len(apiEndpoint)-1] != '/' {
			apiEndpoint += "/"
		}
		discordgo.EndpointAPI = apiEndpoint
		// Override statically generated dependent endpoints
		discordgo.EndpointChannels = discordgo.EndpointAPI + "channels/"
	}

	s, err := discordgo.New("Bot " + botToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	// Disable HTTP Keep-Alives to prevent default clients from holding
	// connections open which causes integration test mock servers to hang.
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DisableKeepAlives = true
	s.Client.Transport = tr

	return &botSender{
		session:         s,
		notifyChannelID: notifyChannelID,
	}, nil
}

func (s *botSender) SendMessage(ctx context.Context, text string) error {
	if text == "" {
		return errors.New("text must not be empty")
	}

	// Split message into chunks if it exceeds Discord's 2000 character limit.
	// We split by newline to avoid breaking formatting where possible.
	const maxLen = 2000
	var chunks []string
	if len(text) <= maxLen {
		chunks = append(chunks, text)
	} else {
		lines := []rune(text)
		for len(lines) > 0 {
			if len(lines) <= maxLen {
				chunks = append(chunks, string(lines))
				break
			}
			// Find the last newline within the maxLen
			splitIdx := -1
			for i := maxLen; i >= 0; i-- {
				if lines[i] == '\n' {
					splitIdx = i
					break
				}
			}
			if splitIdx == -1 {
				// No newline found within limit, force split
				splitIdx = maxLen
			}
			chunks = append(chunks, string(lines[:splitIdx]))
			if splitIdx == maxLen {
				lines = lines[splitIdx:]
			} else {
				// Skip the newline character we split on to avoid leading newlines
				lines = lines[splitIdx+1:]
			}
		}
	}

	for _, chunk := range chunks {
		if chunk == "" {
			continue
		}
		// Using the REST API through session
		_, err := s.session.ChannelMessageSend(s.notifyChannelID, chunk, discordgo.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("failed to send discord message: %w", err)
		}
	}

	return nil
}

func (s *botSender) Close() error {
	return s.session.Close()
}
