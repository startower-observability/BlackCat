package discovery

// truncateBody truncates a response body for error messages.
func truncateBody(body []byte) string {
	const maxLen = 256
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "..."
}
