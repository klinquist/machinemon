package alerting

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/machinemon/machinemon/internal/models"
)

type PushoverProvider struct {
	AppToken string `json:"app_token"`
	UserKey  string `json:"user_key"`
}

func (p *PushoverProvider) Name() string {
	return "pushover"
}

func (p *PushoverProvider) Validate() error {
	if p.AppToken == "" {
		return fmt.Errorf("app_token is required")
	}
	if p.UserKey == "" {
		return fmt.Errorf("user_key is required")
	}
	return nil
}

func (p *PushoverProvider) Send(alert *models.Alert) error {
	priority := "0" // normal
	if alert.Severity == models.SeverityCritical {
		priority = "1" // high
	}

	data := url.Values{}
	data.Set("token", p.AppToken)
	data.Set("user", p.UserKey)
	data.Set("title", fmt.Sprintf("MachineMon %s", strings.ToUpper(alert.Severity)))
	data.Set("message", alert.Message)
	data.Set("priority", priority)

	resp, err := http.PostForm("https://api.pushover.net/1/messages.json", data)
	if err != nil {
		return fmt.Errorf("send pushover: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pushover API error (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}
