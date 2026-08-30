package ntfy

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Notifier struct {
	baseURL string
	topic   string
	token   string
	http    *http.Client
}

const clientTimeout = 10 * time.Second

// token is an optional ntfy bearer token; when empty, requests are unauthenticated.
func NewNotifier(baseURL, topic, token string) *Notifier {
	return &Notifier{
		baseURL: strings.TrimRight(baseURL, "/"),
		topic:   topic,
		token:   token,
		http:    &http.Client{Timeout: clientTimeout},
	}
}

// click is a URL opened when the notification is tapped; tags are ntfy tag/emoji
// shortcodes.
func (n *Notifier) Notify(title, body, click string, tags ...string) error {
	url := fmt.Sprintf("%s/%s", n.baseURL, n.topic)

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return err
	}
	if n.token != "" {
		req.Header.Set("Authorization", "Bearer "+n.token)
	}
	if title != "" {
		req.Header.Set("Title", title)
	}
	if click != "" {
		req.Header.Set("Click", click)
	}
	if len(tags) > 0 {
		req.Header.Set("Tags", strings.Join(tags, ","))
	}

	resp, err := n.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned status %d", resp.StatusCode)
	}
	return nil
}
