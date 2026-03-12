package discord

import (
	"context"
	"testing"
)

// Tests for discordgo sender needs an elaborate mock for discordgo.Session.
// Considering it is a simple wrapper over discordgo, we will use basic checks or mock it differently later.
// For now, T4 (Empty text) can be tested easily.

func TestSendMessage_EmptyText(t *testing.T) {
	sender := &botSender{
		session:         nil, // Should not be accessed
		notifyChannelID: "123",
	}

	err := sender.SendMessage(context.Background(), "")
	if err == nil || err.Error() != "text must not be empty" {
		t.Fatalf("expected 'text must not be empty' error, got %v", err)
	}
}
