package alerting

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/machinemon/machinemon/internal/models"
	"github.com/machinemon/machinemon/internal/store"
)

type Dispatcher struct {
	store  store.Store
	logger *slog.Logger
}

func NewDispatcher(st store.Store, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{store: st, logger: logger}
}

func (d *Dispatcher) Dispatch(alert *models.Alert) error {
	providers, err := d.store.GetEnabledProviders()
	if err != nil {
		return fmt.Errorf("get providers: %w", err)
	}

	if len(providers) == 0 {
		d.logger.Debug("no alert providers configured, skipping dispatch")
		return nil
	}

	var errs []error
	for _, ap := range providers {
		provider, err := d.resolveProvider(ap)
		if err != nil {
			d.logger.Error("failed to resolve provider", "name", ap.Name, "type", ap.Type, "err", err)
			errs = append(errs, fmt.Errorf("provider %s: %w", ap.Name, err))
			continue
		}
		if err := provider.Send(alert); err != nil {
			d.logger.Error("failed to send alert", "provider", ap.Name, "err", err)
			errs = append(errs, fmt.Errorf("provider %s: %w", ap.Name, err))
		} else {
			d.logger.Info("alert sent", "provider", ap.Name, "alert_type", alert.AlertType)
		}
	}

	if len(errs) == 0 {
		d.store.MarkAlertNotified(alert.ID)
	}
	return errors.Join(errs...)
}

func (d *Dispatcher) resolveProvider(ap models.AlertProvider) (Provider, error) {
	switch ap.Type {
	case "twilio":
		var p TwilioProvider
		if err := json.Unmarshal([]byte(ap.Config), &p); err != nil {
			return nil, fmt.Errorf("parse twilio config: %w", err)
		}
		return &p, nil
	case "pushover":
		var p PushoverProvider
		if err := json.Unmarshal([]byte(ap.Config), &p); err != nil {
			return nil, fmt.Errorf("parse pushover config: %w", err)
		}
		return &p, nil
	case "smtp":
		var p SMTPProvider
		if err := json.Unmarshal([]byte(ap.Config), &p); err != nil {
			return nil, fmt.Errorf("parse smtp config: %w", err)
		}
		return &p, nil
	default:
		return nil, fmt.Errorf("unknown provider type: %s", ap.Type)
	}
}

// SendTestAlert sends a test notification through a specific provider.
func (d *Dispatcher) SendTestAlert(providerID int64) (*models.TestAlertResult, error) {
	ap, err := d.store.GetProvider(providerID)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}
	if ap == nil {
		return nil, fmt.Errorf("provider not found")
	}

	provider, err := d.resolveProvider(*ap)
	if err != nil {
		return nil, err
	}
	if err := provider.Validate(); err != nil {
		return nil, fmt.Errorf("invalid %s config: %w", ap.Type, err)
	}

	testAlert := &models.Alert{
		AlertType: "test",
		Severity:  models.SeverityInfo,
		Message:   "This is a test alert from MachineMon.",
	}

	if p, ok := provider.(*PushoverProvider); ok {
		sendResult, err := p.send(testAlert)
		if err != nil {
			return nil, err
		}

		msg := fmt.Sprintf("Pushover accepted test alert (HTTP %d", sendResult.HTTPStatusCode)
		if sendResult.APIStatus != 0 {
			msg += fmt.Sprintf(", status=%d", sendResult.APIStatus)
		}
		if sendResult.RequestID != "" {
			msg += fmt.Sprintf(", request=%s", sendResult.RequestID)
		}
		msg += ")"

		return &models.TestAlertResult{
			Provider:      ap.Type,
			Message:       msg,
			APIStatusCode: sendResult.HTTPStatusCode,
			APIResponse:   sendResult.RawResponse,
		}, nil
	}

	if err := provider.Send(testAlert); err != nil {
		return nil, err
	}
	return &models.TestAlertResult{
		Provider: ap.Type,
		Message:  fmt.Sprintf("%s accepted test alert", ap.Type),
	}, nil
}
