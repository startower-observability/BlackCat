//go:build cgo

// Package whatsapp implements a WhatsApp channel adapter using the unofficial
// whatsmeow library (go.mau.fi/whatsmeow). This is NOT an official WhatsApp
// Business API integration — it uses the WhatsApp Web multi-device protocol
// directly. Use at your own risk; WhatsApp may ban accounts using unofficial APIs.
//
// Requirements:
//   - CGO enabled (whatsmeow uses SQLite for session persistence)
//   - A phone with WhatsApp to scan the QR code on first login
package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/startower-observability/blackcat/internal/channel"
	itypes "github.com/startower-observability/blackcat/internal/types"
)

func init() {
	// Show as "Google Chrome" in WhatsApp's Linked Devices list.
	// Must be set before any whatsmeow.NewClient() call.
	store.DeviceProps.Os = proto.String("Google Chrome")
	store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_CHROME.Enum()
}

// dedupeCache prevents duplicate processing of the same WhatsApp message.
// whatsmeow can fire the same Message event multiple times (e.g. on reconnect).
type dedupeCache struct {
	mu      sync.Mutex
	seen    map[string]time.Time
	maxSize int
	ttl     time.Duration
}

func newDedupeCache(maxSize int, ttl time.Duration) *dedupeCache {
	return &dedupeCache{
		seen:    make(map[string]time.Time, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// check returns true if the key was already seen (duplicate).
// If not seen, records the key and returns false.
func (d *dedupeCache) check(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	// Evict expired entries
	for k, t := range d.seen {
		if now.Sub(t) > d.ttl {
			delete(d.seen, k)
		}
	}
	if _, exists := d.seen[key]; exists {
		return true
	}
	// Cap size: evict oldest entry if at limit
	if len(d.seen) >= d.maxSize {
		var oldest string
		var oldestTime time.Time
		for k, t := range d.seen {
			if oldest == "" || t.Before(oldestTime) {
				oldest = k
				oldestTime = t
			}
		}
		delete(d.seen, oldest)
	}
	d.seen[key] = now
	return false
}

// WhatsAppChannel implements types.Channel using whatsmeow.
type WhatsAppChannel struct {
	client    *whatsmeow.Client
	storePath string
	allowFrom map[string]bool // normalised E.164 → true; nil = allow all
	incoming  chan itypes.Message
	qrOut     chan<- string
	mu        sync.Mutex
	started   bool
	cancel    context.CancelFunc
	dedup     *dedupeCache
}

// NewWhatsAppChannel creates a new WhatsApp channel adapter.
// storePath is the path to the SQLite database for whatsmeow session persistence.
// allowFrom is an optional phone whitelist in E.164 format (e.g. "+628123456789").
// If empty or nil, all senders are allowed. A single "*" entry explicitly allows all.
func NewWhatsAppChannel(storePath string, allowFrom []string) *WhatsAppChannel {
	ch := &WhatsAppChannel{
		storePath: storePath,
		incoming:  make(chan itypes.Message, 256),
		dedup:     newDedupeCache(5000, 20*time.Minute),
	}
	if len(allowFrom) > 0 {
		ch.allowFrom = make(map[string]bool, len(allowFrom))
		for _, phone := range allowFrom {
			if phone == "*" {
				ch.allowFrom = nil // wildcard = allow all
				break
			}
			ch.allowFrom[normalizeE164(phone)] = true
		}
	}
	return ch
}

func (w *WhatsAppChannel) SetQRChannel(ch chan<- string) {
	w.mu.Lock()
	w.qrOut = ch
	w.mu.Unlock()
}

// Start initialises the whatsmeow client, connects to WhatsApp, and begins
// listening for incoming messages. If no session exists, a QR code is logged
// via slog for the user to scan from their phone.
func (w *WhatsAppChannel) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.started {
		return fmt.Errorf("whatsapp channel already started")
	}

	// Initialise whatsmeow SQLite store for session persistence.
	container, err := sqlstore.New(ctx, "sqlite3", w.storePath, nil)
	if err != nil {
		return fmt.Errorf("whatsapp: failed to open store %q: %w", w.storePath, err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("whatsapp: failed to get device: %w", err)
	}

	client := whatsmeow.NewClient(deviceStore, nil)
	w.client = client

	// Register event handler for incoming messages and connection events.
	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Connected:
			// Set online presence so the bot appears as "online" in contacts.
			if presErr := client.SendPresence(context.Background(), types.PresenceAvailable); presErr != nil {
				slog.Warn("whatsapp: failed to set online presence", "err", presErr)
			} else {
				slog.Info("whatsapp: online presence set")
			}
		case *events.Message:
			// Deduplicate: whatsmeow can fire the same event more than once.
			dedupeKey := v.Info.Sender.String() + "|" + v.Info.ID
			if w.dedup.check(dedupeKey) {
				slog.Debug("whatsapp: duplicate message dropped", "id", v.Info.ID)
				return
			}
			msg, ok := convertEvent(v)
			if !ok {
				return
			}
			resolvedPhone := w.resolveSenderPhone(v)
			if !w.isAllowed(v.Info.Chat.String(), v.Info.Sender.String(), resolvedPhone) {
				slog.Info("whatsapp: message from non-allowed sender, dropping",
					"chat", v.Info.Chat.String(), "sender", v.Info.Sender.String(),
					"resolvedPhone", resolvedPhone)
				return
			}
			select {
			case w.incoming <- msg:
				// Show "typing..." while the message is being processed.
				_ = client.SendChatPresence(context.Background(), v.Info.Chat,
					types.ChatPresenceComposing, types.ChatPresenceMediaText)
			default:
				slog.Warn("whatsapp: incoming buffer full, dropping message",
					"id", msg.ID, "from", msg.UserID)
			}
		}
	})

	// Connect — if not logged in, generate QR code for pairing.
	if client.Store.ID == nil {
		slog.Info("whatsapp: no existing session, generating QR code for login")
		qrChan, _ := client.GetQRChannel(ctx)
		if err := client.Connect(); err != nil {
			return fmt.Errorf("whatsapp: connect failed: %w", err)
		}
		// Process QR events in background.
		go func() {
			for evt := range qrChan {
				if evt.Event == "code" {
					w.mu.Lock()
					qrOut := w.qrOut
					w.mu.Unlock()
					if qrOut != nil {
						select {
						case qrOut <- evt.Code:
						default:
						}
					}
					slog.Info("whatsapp: scan this QR code to log in", "qr", evt.Code)
				} else {
					slog.Info("whatsapp: login event", "event", evt.Event)
				}
			}
		}()
	} else {
		slog.Info("whatsapp: reconnecting with existing session")
		if err := client.Connect(); err != nil {
			return fmt.Errorf("whatsapp: connect failed: %w", err)
		}
	}

	ctx, w.cancel = context.WithCancel(ctx)
	w.started = true

	// Disconnect when context is cancelled.
	go func() {
		<-ctx.Done()
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.client != nil {
			w.client.Disconnect()
		}
	}()

	slog.Info("whatsapp: channel started")
	return nil
}

// Stop disconnects the client and closes the incoming channel.
func (w *WhatsAppChannel) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.started {
		return nil
	}

	if w.cancel != nil {
		w.cancel()
	}
	if w.client != nil {
		w.client.Disconnect()
		w.client = nil
	}

	close(w.incoming)
	w.started = false
	slog.Info("whatsapp: channel stopped")
	return nil
}

// Send delivers a text message to the WhatsApp chat identified by msg.ChannelID.
// Long messages are split into human-like bubbles with delays between them.
func (w *WhatsAppChannel) Send(ctx context.Context, msg itypes.Message) error {
	w.mu.Lock()
	client := w.client
	started := w.started
	w.mu.Unlock()

	if !started || client == nil {
		return fmt.Errorf("whatsapp: channel not started")
	}

	jid, err := types.ParseJID(msg.ChannelID)
	if err != nil {
		return fmt.Errorf("whatsapp: invalid JID %q: %w", msg.ChannelID, err)
	}

	formattedContent := FormatForWhatsApp(msg.Content)
	bubbles := channel.SplitBubbles(formattedContent, 5, 4096)

	// Set typing indicator ONCE before sending all bubbles.
	_ = client.SendChatPresence(ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaText)

	totalDelay := time.Duration(0)
	const maxTotalDelay = 10 * time.Second

	for i, bubble := range bubbles {
		// Add delay between bubbles (NOT before the first one).
		if i > 0 && totalDelay < maxTotalDelay {
			delay := randomDelay(300, 800)
			if totalDelay+delay > maxTotalDelay {
				delay = maxTotalDelay - totalDelay
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			totalDelay += delay
		}

		_, err := client.SendMessage(ctx, jid, &waE2E.Message{
			Conversation: proto.String(bubble),
		})
		if err != nil {
			return fmt.Errorf("whatsapp: send failed: %w", err)
		}
	}

	// Clear typing indicator after sending all bubbles.
	_ = client.SendChatPresence(ctx, jid, types.ChatPresencePaused, types.ChatPresenceMediaText)
	return nil
}

// Receive returns the read-only channel of incoming messages.
func (w *WhatsAppChannel) Receive() <-chan itypes.Message {
	return w.incoming
}

// Info returns metadata about this channel.
func (w *WhatsAppChannel) Info() itypes.ChannelInfo {
	w.mu.Lock()
	defer w.mu.Unlock()
	return itypes.ChannelInfo{
		Type:      itypes.ChannelWhatsApp,
		Name:      "whatsapp",
		Connected: w.started,
	}
}

// Health checks the health of the WhatsApp channel.
func (w *WhatsAppChannel) Health() itypes.ChannelHealth {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.started || w.client == nil {
		return itypes.ChannelHealth{
			Name:    "whatsapp",
			Healthy: false,
			Details: "channel not started",
		}
	}

	// Check if connected
	if !w.client.IsConnected() {
		return itypes.ChannelHealth{
			Name:    "whatsapp",
			Healthy: false,
			Details: "not connected to WhatsApp",
		}
	}

	return itypes.ChannelHealth{
			Name:    "whatsapp",
			Healthy: true,
			Details: "connected and responsive",
		}
}

// Reconnect attempts to reconnect the WhatsApp client.
// Implements types.Reconnectable for heartbeat auto-recovery.
func (w *WhatsAppChannel) Reconnect(_ context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.client == nil {
		return fmt.Errorf("whatsapp: no client available")
	}

	if w.client.IsConnected() {
		return nil // already connected
	}

	slog.Info("whatsapp: attempting reconnect")
	return w.client.Connect()
}

// compile-time interface check for Reconnectable
var _ itypes.Reconnectable = (*WhatsAppChannel)(nil)

// convertEvent extracts a types.Message from a whatsmeow Message event.
// Returns false if the event contains no text content.
func convertEvent(evt *events.Message) (itypes.Message, bool) {
	text := evt.Message.GetConversation()
	if text == "" {
		if ext := evt.Message.GetExtendedTextMessage(); ext != nil {
			text = ext.GetText()
		}
	}

	// Handle image/video/document messages — extract caption only (no binary processing)
	if text == "" {
		if img := evt.Message.GetImageMessage(); img != nil {
			text = img.GetCaption()
			if text == "" {
				text = "[Image received — no caption provided]"
			}
		}
	}
	if text == "" {
		if vid := evt.Message.GetVideoMessage(); vid != nil {
			text = vid.GetCaption()
			if text == "" {
				text = "[Video received — no caption provided]"
			}
		}
	}
	if text == "" {
		if doc := evt.Message.GetDocumentMessage(); doc != nil {
			text = doc.GetCaption()
			if text == "" {
				text = fmt.Sprintf("[Document received: %s]", doc.GetFileName())
			}
		}
	}

	if text == "" {
		return itypes.Message{}, false
	}

	// Extract quoted message content for reply context
	var quotedText string
	if ext := evt.Message.GetExtendedTextMessage(); ext != nil {
		if ctxInfo := ext.GetContextInfo(); ctxInfo != nil {
			if quoted := ctxInfo.GetQuotedMessage(); quoted != nil {
				quotedText = quoted.GetConversation()
				if quotedText == "" {
					if qExt := quoted.GetExtendedTextMessage(); qExt != nil {
						quotedText = qExt.GetText()
					}
				}
			}
		}
	}

	content := text
	if quotedText != "" {
		content = fmt.Sprintf("[Replying to: %s]\n\n%s", quotedText, text)
	}

	return itypes.Message{
		ID:          evt.Info.ID,
		ChannelType: itypes.ChannelWhatsApp,
		ChannelID:   evt.Info.Chat.String(),
		UserID:      evt.Info.Sender.String(),
		Content:     content,
		Timestamp:   evt.Info.Timestamp,
	}, true
}

// randomDelay returns a random duration between minMs and maxMs milliseconds.
func randomDelay(minMs, maxMs int) time.Duration {
	return time.Duration(minMs+rand.Intn(maxMs-minMs)) * time.Millisecond
}

// isAllowed checks whether a message sender passes the phone whitelist.
// resolvedPhone is the E.164 phone number resolved from LID (may be empty).
// It also checks the chat and sender JIDs as fallback for older protocol.
func (w *WhatsAppChannel) isAllowed(chatJID, senderJID, resolvedPhone string) bool {
	if w.allowFrom == nil {
		return true
	}
	// Prefer resolved phone from LID resolution (most reliable).
	if resolvedPhone != "" && w.allowFrom[resolvedPhone] {
		return true
	}
	// Chat JID uses phone format in DMs (e.g. 6282394921432@s.whatsapp.net)
	if phone := phoneFromJID(chatJID); phone != "" && w.allowFrom[phone] {
		return true
	}
	// Fallback: check sender JID (may also be phone-based in older protocol)
	if phone := phoneFromJID(senderJID); phone != "" && w.allowFrom[phone] {
		return true
	}
	return false
}

// resolveSenderPhone resolves the sender's phone number from a message event.
// WhatsApp has migrated to LID (Linked IDs) where sender JIDs use opaque IDs
// like "249271429374089@lid" instead of phone-based JIDs. This resolves the
// LID back to a phone number using:
//  1. Direct phone JID (sender is already phone-based)
//  2. SenderAlt field (inline cross-reference from wire protocol)
//  3. DB lookup via Store.LIDs.GetPNForLID (fallback)
func (w *WhatsAppChannel) resolveSenderPhone(evt *events.Message) string {
	info := evt.Info
	// Sender is already phone-based (s.whatsapp.net).
	if info.Sender.Server == types.DefaultUserServer {
		return "+" + info.Sender.ToNonAD().User
	}
	// Sender is LID but SenderAlt carries the phone JID.
	if !info.SenderAlt.IsEmpty() && info.SenderAlt.Server == types.DefaultUserServer {
		return "+" + info.SenderAlt.ToNonAD().User
	}
	// DB fallback: whatsmeow stores LID↔PN mappings from message history.
	if w.client != nil {
		pn, err := w.client.Store.LIDs.GetPNForLID(context.Background(), info.Sender.ToNonAD())
		if err == nil && !pn.IsEmpty() {
			return "+" + pn.User
		}
	}
	return ""
}

// phoneFromJID extracts a normalised E.164 phone number from a WhatsApp JID.
// Handles device suffix: "6282394921432:5@s.whatsapp.net" → "+6282394921432"
func phoneFromJID(jid string) string {
	user, _, _ := strings.Cut(jid, "@")
	if user == "" {
		return ""
	}
	// Strip device suffix (e.g. ":5", ":0")
	if idx := strings.IndexByte(user, ':'); idx > 0 {
		user = user[:idx]
	}
	return "+" + user
}

// normalizeE164 normalises a phone string to E.164 format by stripping
// everything except digits and ensuring a leading "+".
// Example: "+62 812-345-6789" → "+628123456789"
func normalizeE164(phone string) string {
	var buf strings.Builder
	for i, r := range phone {
		if r == '+' && i == 0 {
			buf.WriteRune(r)
		} else if r >= '0' && r <= '9' {
			buf.WriteRune(r)
		}
	}
	s := buf.String()
	if !strings.HasPrefix(s, "+") {
		s = "+" + s
	}
	return s
}
