package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var channelsSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a message to a channel via the running daemon",
	Long: `Send a text message through a connected channel (WhatsApp, Telegram, Discord)
by calling the daemon's /api/channels/send endpoint.

The daemon must be running. The target channel must be connected.

Examples:
  blackcat channels send --channel whatsapp --to 6283111043962@s.whatsapp.net --message "Selamat Pagi"
  blackcat channels send --channel telegram --to 123456789 --message "Hello from BlackCat"
  blackcat channels send --channel discord --to 1234567890123456 --message "Hello"`,
	RunE: runChannelsSend,
}

func init() {
	channelsCmd.AddCommand(channelsSendCmd)
	channelsSendCmd.Flags().String("channel", "", "Channel type: whatsapp, telegram, discord")
	channelsSendCmd.Flags().String("to", "", "Channel ID / chat ID to send to (e.g. JID for WhatsApp, chat ID for Telegram)")
	channelsSendCmd.Flags().String("message", "", "Message text to send")
	_ = channelsSendCmd.MarkFlagRequired("channel")
	_ = channelsSendCmd.MarkFlagRequired("to")
	_ = channelsSendCmd.MarkFlagRequired("message")
}

func runChannelsSend(cmd *cobra.Command, args []string) error {
	channelFlag, _ := cmd.Flags().GetString("channel")
	channelFlag = strings.ToLower(strings.TrimSpace(channelFlag))
	to, _ := cmd.Flags().GetString("to")
	to = strings.TrimSpace(to)
	message, _ := cmd.Flags().GetString("message")

	switch channelFlag {
	case "whatsapp", "telegram", "discord":
	default:
		return fmt.Errorf("unknown channel: %s (available: whatsapp, telegram, discord)", channelFlag)
	}

	// For WhatsApp, auto-append @s.whatsapp.net if missing.
	if channelFlag == "whatsapp" && !strings.Contains(to, "@") {
		to = to + "@s.whatsapp.net"
	}

	payload := map[string]string{
		"channel":   channelFlag,
		"channelId": to,
		"message":   message,
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post("http://127.0.0.1:8080/api/channels/send", "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Println("Daemon not running or not reachable. Start with: blackcat daemon")
		return nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("send failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	fmt.Printf("Message sent to %s via %s\n", to, channelFlag)
	return nil
}
