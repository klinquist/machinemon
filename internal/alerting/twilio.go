package alerting

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/machinemon/machinemon/internal/models"
)

type TwilioProvider struct {
	AccountSID string `json:"account_sid"`
	AuthToken  string `json:"auth_token"`
	FromNumber string `json:"from_number"`
	ToNumber   string `json:"to_number"`
}

func (t *TwilioProvider) Name() string {
	return "twilio"
}

func (t *TwilioProvider) Validate() error {
	if t.AccountSID == "" {
		return fmt.Errorf("account_sid is required")
	}
	if t.AuthToken == "" {
		return fmt.Errorf("auth_token is required")
	}
	if t.FromNumber == "" {
		return fmt.Errorf("from_number is required")
	}
	if t.ToNumber == "" {
		return fmt.Errorf("to_number is required")
	}
	return nil
}

func (t *TwilioProvider) Send(alert *models.Alert) error {
	body := fmt.Sprintf("[MachineMon %s] %s", strings.ToUpper(alert.Severity), alert.Message)

	data := url.Values{}
	data.Set("To", t.ToNumber)
	data.Set("From", t.FromNumber)
	data.Set("Body", body)

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.AccountSID)
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth(t.AccountSID, t.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send SMS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twilio API error (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}
