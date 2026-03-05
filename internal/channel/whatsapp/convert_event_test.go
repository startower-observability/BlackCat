//go:build cgo

package whatsapp

import (
	"strings"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// helper to build a minimal *events.Message with the given waE2E.Message.
func makeEvent(msg *waE2E.Message) *events.Message {
	return &events.Message{
		Info: types.MessageInfo{
			ID:        "test-msg-id",
			Chat:      types.JID{User: "628123456789", Server: "s.whatsapp.net"},
			Sender:    types.JID{User: "628123456789", Server: "s.whatsapp.net"},
			Timestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		Message: msg,
	}
}

func TestConvertEventWithoutQuotedMessage(t *testing.T) {
	evt := makeEvent(&waE2E.Message{
		Conversation: proto.String("Hello world"),
	})

	msg, ok := convertEvent(evt)
	if !ok {
		t.Fatal("expected ok=true for plain text message")
	}
	if msg.Content != "Hello world" {
		t.Fatalf("expected content %q, got %q", "Hello world", msg.Content)
	}
	if msg.ID != "test-msg-id" {
		t.Fatalf("expected ID %q, got %q", "test-msg-id", msg.ID)
	}
}

func TestConvertEventWithQuotedMessage(t *testing.T) {
	evt := makeEvent(&waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("My reply"),
			ContextInfo: &waE2E.ContextInfo{
				QuotedMessage: &waE2E.Message{
					Conversation: proto.String("Original message"),
				},
			},
		},
	})

	msg, ok := convertEvent(evt)
	if !ok {
		t.Fatal("expected ok=true for reply message")
	}
	if !strings.HasPrefix(msg.Content, "[Replying to: Original message]") {
		t.Fatalf("expected reply prefix, got %q", msg.Content)
	}
	if !strings.HasSuffix(msg.Content, "My reply") {
		t.Fatalf("expected reply text at end, got %q", msg.Content)
	}
	expected := "[Replying to: Original message]\n\nMy reply"
	if msg.Content != expected {
		t.Fatalf("expected content %q, got %q", expected, msg.Content)
	}
}

func TestConvertEventWithQuotedExtendedText(t *testing.T) {
	// Quoted message is an ExtendedTextMessage (not plain Conversation)
	evt := makeEvent(&waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String("Replying here"),
			ContextInfo: &waE2E.ContextInfo{
				QuotedMessage: &waE2E.Message{
					ExtendedTextMessage: &waE2E.ExtendedTextMessage{
						Text: proto.String("Quoted extended text"),
					},
				},
			},
		},
	})

	msg, ok := convertEvent(evt)
	if !ok {
		t.Fatal("expected ok=true")
	}
	expected := "[Replying to: Quoted extended text]\n\nReplying here"
	if msg.Content != expected {
		t.Fatalf("expected content %q, got %q", expected, msg.Content)
	}
}

func TestConvertEventWithImageCaption(t *testing.T) {
	evt := makeEvent(&waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption: proto.String("Check out this photo"),
		},
	})

	msg, ok := convertEvent(evt)
	if !ok {
		t.Fatal("expected ok=true for image with caption")
	}
	if msg.Content != "Check out this photo" {
		t.Fatalf("expected caption content, got %q", msg.Content)
	}
}

func TestConvertEventWithImageNoCaption(t *testing.T) {
	evt := makeEvent(&waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{},
	})

	msg, ok := convertEvent(evt)
	if !ok {
		t.Fatal("expected ok=true for image without caption")
	}
	if msg.Content != "[Image received — no caption provided]" {
		t.Fatalf("expected placeholder, got %q", msg.Content)
	}
}

func TestConvertEventWithVideoCaption(t *testing.T) {
	evt := makeEvent(&waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{
			Caption: proto.String("Watch this video"),
		},
	})

	msg, ok := convertEvent(evt)
	if !ok {
		t.Fatal("expected ok=true for video with caption")
	}
	if msg.Content != "Watch this video" {
		t.Fatalf("expected caption content, got %q", msg.Content)
	}
}

func TestConvertEventWithVideoNoCaption(t *testing.T) {
	evt := makeEvent(&waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{},
	})

	msg, ok := convertEvent(evt)
	if !ok {
		t.Fatal("expected ok=true for video without caption")
	}
	if msg.Content != "[Video received — no caption provided]" {
		t.Fatalf("expected placeholder, got %q", msg.Content)
	}
}

func TestConvertEventWithDocumentCaption(t *testing.T) {
	evt := makeEvent(&waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			Caption:  proto.String("Here is the report"),
			FileName: proto.String("report.pdf"),
		},
	})

	msg, ok := convertEvent(evt)
	if !ok {
		t.Fatal("expected ok=true for document with caption")
	}
	if msg.Content != "Here is the report" {
		t.Fatalf("expected caption content, got %q", msg.Content)
	}
}

func TestConvertEventWithDocument(t *testing.T) {
	evt := makeEvent(&waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			FileName: proto.String("report.pdf"),
		},
	})

	msg, ok := convertEvent(evt)
	if !ok {
		t.Fatal("expected ok=true for document with filename")
	}
	expected := "[Document received: report.pdf]"
	if msg.Content != expected {
		t.Fatalf("expected %q, got %q", expected, msg.Content)
	}
}

func TestConvertEventEmptyMessage(t *testing.T) {
	evt := makeEvent(&waE2E.Message{})

	_, ok := convertEvent(evt)
	if ok {
		t.Fatal("expected ok=false for empty message")
	}
}
