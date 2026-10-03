package watch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Payload is the JSON body sent to generic webhooks.
type Payload struct {
	Event     string    `json:"event"` // always "acurl.changes"
	Timestamp time.Time `json:"timestamp"`
	Changes   []Change  `json:"changes"`
}

// Notifier posts change alerts to a webhook.
type Notifier struct {
	URL string
	// Format is "json" (default) or "slack" (Slack incoming webhook, also
	// accepted by Mattermost, Discord's /slack endpoint and others).
	Format string
	Client *http.Client
}

// Notify sends changes. It does nothing when there are none.
func (n *Notifier) Notify(ctx context.Context, changes []Change) error {
	if n == nil || n.URL == "" || len(changes) == 0 {
		return nil
	}
	var body any
	switch n.Format {
	case "slack":
		body = map[string]string{"text": slackText(changes)}
	case "", "json":
		body = Payload{Event: "acurl.changes", Timestamp: time.Now().UTC(), Changes: changes}
	default:
		return fmt.Errorf("watch: unknown webhook format %q", n.Format)
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "acurl-watch (+https://github.com/tragent-io/acurl)")
	client := n.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("watch: webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("watch: webhook returned %d", resp.StatusCode)
	}
	return nil
}

func slackText(changes []Change) string {
	var sb strings.Builder
	sb.WriteString("*acurl:* AI agent access changed\n")
	for _, c := range changes {
		icon := ":white_check_mark:"
		if c.Worse {
			icon = ":warning:"
		}
		fmt.Fprintf(&sb, "%s %s\n", icon, c.String())
	}
	return sb.String()
}
