package alerting

import "github.com/machinemon/machinemon/internal/models"

// Provider sends alert notifications to an external service.
type Provider interface {
	Send(alert *models.Alert) error
	Validate() error
	Name() string
}
