package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Sink implementations.
//
// These use net/http directly rather than vendor SDKs. The payloads are small
// and stable, and a scan binary that stays dependency-free in its analysis path
// is worth more than the convenience: `drift scan` must not get slower or
// larger because a notification sink exists.

// httpTimeout bounds every outbound call. A sink that hangs would hold up a CI
// job long after the scan it was reporting has finished.
const httpTimeout = 15 * time.Second

// StdoutSink prints notifications as text. It is the default, and the one used
// when no delivery is configured, so `infractl notify` always does something
// observable rather than silently succeeding.
type StdoutSink struct {
	Out io.Writer
}

// Name identifies the sink.
func (StdoutSink) Name() string { return "stdout" }

// Send writes the notification as plain text.
func (s StdoutSink) Send(_ context.Context, n Notification) error {
	var b strings.Builder

	fmt.Fprintf(&b, "[%s] %s\n", strings.ToUpper(string(n.Tier)), n.Title)
	fmt.Fprintf(&b, "%s\n", n.Summary)

	for _, item := range n.Items {
		fmt.Fprintf(&b, "  %-9s %s", strings.ToUpper(item.Severity), item.Address)
		if item.Age != "" {
			fmt.Fprintf(&b, " (open %s)", item.Age)
		}
		b.WriteString("\n")
		if len(item.Paths) > 0 {
			fmt.Fprintf(&b, "            %s\n", strings.Join(item.Paths, ", "))
		}
	}

	if len(n.Actions) > 0 {
		b.WriteString("\n")
		for _, action := range n.Actions {
			fmt.Fprintf(&b, "  %-16s %s\n", action.Label, action.Command)
		}
	}

	_, err := io.WriteString(s.Out, b.String()+"\n")
	return err
}

// SlackSink posts to a Slack channel via chat.postMessage.
//
// Scopes required: chat:write. Nothing else. Notably not channels:history,
// which would let a compromised token read the channel it posts to.
type SlackSink struct {
	Token   string
	Channel string
	Client  *http.Client
}

// Name identifies the sink.
func (SlackSink) Name() string { return "slack" }

// Send posts a Block Kit message.
func (s SlackSink) Send(ctx context.Context, n Notification) error {
	if s.Token == "" {
		return fmt.Errorf("slack sink has no token; set the token environment variable named in your config")
	}
	if s.Channel == "" {
		return fmt.Errorf("slack sink has no channel")
	}

	payload := map[string]any{
		"channel": s.Channel,
		// A text fallback is required for notifications and accessibility
		// clients that do not render blocks.
		"text":   n.Title,
		"blocks": slackBlocks(n),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+s.Token)

	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: httpTimeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post to slack: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Slack answers 200 with ok:false for application errors, so the status
	// code alone does not tell you whether the message was delivered.
	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return fmt.Errorf("decode slack response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("slack rejected the message: %s", result.Error)
	}
	return nil
}

// slackBlocks renders a notification as Block Kit.
//
// Every string here has already been through Sanitise, so a resource named
// after a mention cannot become one.
func slackBlocks(n Notification) []map[string]any {
	blocks := []map[string]any{
		{
			"type": "header",
			"text": map[string]any{"type": "plain_text", "text": n.Title},
		},
		{
			"type": "section",
			"text": map[string]any{"type": "mrkdwn", "text": n.Summary},
		},
	}

	// Items are chunked because Slack rejects a block over 3000 characters,
	// and a scan of a large estate will exceed it.
	const maxItemsPerBlock = 10
	for start := 0; start < len(n.Items); start += maxItemsPerBlock {
		end := start + maxItemsPerBlock
		if end > len(n.Items) {
			end = len(n.Items)
		}

		var lines strings.Builder
		for _, item := range n.Items[start:end] {
			fmt.Fprintf(&lines, "`%s` *%s*", item.Address, strings.ToUpper(item.Severity))
			if item.Age != "" {
				fmt.Fprintf(&lines, " open %s", item.Age)
			}
			if len(item.Paths) > 0 {
				fmt.Fprintf(&lines, "\n%s", strings.Join(item.Paths, ", "))
			}
			lines.WriteString("\n")
		}

		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{"type": "mrkdwn", "text": lines.String()},
		})
	}

	if len(n.Actions) > 0 {
		var actions strings.Builder
		for _, action := range n.Actions {
			fmt.Fprintf(&actions, "*%s*  `%s`\n", action.Label, action.Command)
		}
		blocks = append(blocks,
			map[string]any{"type": "divider"},
			map[string]any{
				"type": "section",
				"text": map[string]any{"type": "mrkdwn", "text": actions.String()},
			})
	}

	return blocks
}

// WebhookSink posts signed JSON to an arbitrary endpoint.
type WebhookSink struct {
	URL string
	// Secret signs the body so the receiver can verify origin. Without it the
	// endpoint has no way to distinguish this tool from anyone who learned the
	// URL.
	Secret string
	Client *http.Client
}

// Name identifies the sink.
func (WebhookSink) Name() string { return "webhook" }

// SignatureHeader carries the HMAC of the request body.
const SignatureHeader = "X-Infractl-Signature"

// TimestampHeader carries the send time, so a receiver can reject replays.
const TimestampHeader = "X-Infractl-Timestamp"

// Send posts the notification as JSON.
func (s WebhookSink) Send(ctx context.Context, n Notification) error {
	if s.URL == "" {
		return fmt.Errorf("webhook sink has no url")
	}

	body, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("encode webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if s.Secret != "" {
		timestamp := fmt.Sprintf("%d", time.Now().UTC().Unix())
		req.Header.Set(TimestampHeader, timestamp)
		req.Header.Set(SignatureHeader, Sign(s.Secret, timestamp, body))
	}

	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: httpTimeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post to webhook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("webhook returned %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
	}
	return nil
}

// Sign computes the HMAC a receiver should verify.
//
// The timestamp is inside the signed material so that a captured request
// cannot be replayed later with its signature intact.
func Sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v1:"))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(":"))
	mac.Write(body)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

// Verify checks a signature in constant time.
//
// Exported so that a receiver written in Go can share the implementation
// rather than reimplement it, and because a comparison written with == is the
// classic way this check is got wrong.
func Verify(secret, timestamp, signature string, body []byte) bool {
	expected := Sign(secret, timestamp, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// GitHubSink posts a comment on a pull request.
type GitHubSink struct {
	Token string
	// Repo is "owner/name".
	Repo string
	// PR is the pull request number.
	PR     int
	Client *http.Client
}

// Name identifies the sink.
func (GitHubSink) Name() string { return "github" }

// Send posts the notification as a PR comment.
func (s GitHubSink) Send(ctx context.Context, n Notification) error {
	if s.Token == "" {
		return fmt.Errorf("github sink has no token")
	}
	if s.Repo == "" || s.PR == 0 {
		return fmt.Errorf("github sink needs a repo and a pull request number")
	}

	payload, err := json.Marshal(map[string]string{"body": githubMarkdown(n)})
	if err != nil {
		return fmt.Errorf("encode github payload: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/comments", s.Repo, s.PR)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build github request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: httpTimeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post to github: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("github returned %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
	}
	return nil
}

// githubMarkdown renders a notification as a PR comment.
func githubMarkdown(n Notification) string {
	var b strings.Builder

	fmt.Fprintf(&b, "### %s\n\n%s\n\n", n.Title, n.Summary)

	if len(n.Items) > 0 {
		b.WriteString("| Severity | Resource | Changed |\n|---|---|---|\n")
		for _, item := range n.Items {
			changed := strings.Join(item.Paths, ", ")
			if changed == "" {
				changed = "-"
			}
			// Pipes inside a cell would break the table, and a resource name is
			// attacker-influenced.
			fmt.Fprintf(&b, "| %s | `%s` | %s |\n",
				strings.ToUpper(item.Severity),
				strings.ReplaceAll(item.Address, "|", "\\|"),
				strings.ReplaceAll(changed, "|", "\\|"))
		}
		b.WriteString("\n")
	}

	if len(n.Actions) > 0 {
		b.WriteString("<details><summary>What to do</summary>\n\n```bash\n")
		for _, action := range n.Actions {
			fmt.Fprintf(&b, "# %s\n%s\n\n", action.Label, action.Command)
		}
		b.WriteString("```\n</details>\n")
	}

	b.WriteString("\n<sub>Posted by infractl. Nothing has been applied.</sub>\n")
	return b.String()
}
