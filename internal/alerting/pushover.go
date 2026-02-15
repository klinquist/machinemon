package alerting

import (
	"encoding/json"
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

type pushoverAPIResponse struct {
	Status  int      `json:"status"`
	Request string   `json:"request"`
	Errors  []string `json:"errors"`
}

type PushoverSendResult struct {
	HTTPStatusCode int
	APIStatus      int
	RequestID      string
	RawResponse    string
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
	_, err := p.send(alert)
	return err
}

func (p *PushoverProvider) send(alert *models.Alert) (*PushoverSendResult, error) {
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
		return nil, fmt.Errorf("send pushover: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	raw := strings.TrimSpace(string(respBody))
	result := &PushoverSendResult{
		HTTPStatusCode: resp.StatusCode,
		RawResponse:    raw,
	}

	var apiResp pushoverAPIResponse
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &apiResp); err == nil {
			result.APIStatus = apiResp.Status
			result.RequestID = apiResp.Request
		}
	}

	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("pushover API error (status %d): %s", resp.StatusCode, raw)
	}
	if apiResp.Status != 1 {
		if len(apiResp.Errors) > 0 {
			return result, fmt.Errorf("pushover API rejected request: %s", strings.Join(apiResp.Errors, "; "))
		}
		if raw != "" {
			return result, fmt.Errorf("pushover API rejected request: %s", raw)
		}
		return result, fmt.Errorf("pushover API rejected request")
	}
	return result, nil
}
