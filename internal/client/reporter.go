package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/machinemon/machinemon/internal/models"
	"github.com/machinemon/machinemon/internal/version"
)

type Reporter struct {
	httpClient *http.Client
	serverURL  string
	password   string
}

func NewReporter(serverURL, password string, insecureSkipTLS bool) *Reporter {
	transport := &http.Transport{}
	if insecureSkipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Reporter{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		serverURL: serverURL,
		password:  password,
	}
}

func (r *Reporter) CheckIn(clientID string, metrics *SystemMetrics, procs []ProcessStatus, checks []CheckResult) (*models.CheckInResponse, error) {
	hostname, _ := os.Hostname()

	processes := make([]models.ProcessPayload, len(procs))
	for i, p := range procs {
		processes[i] = models.ProcessPayload{
			FriendlyName: p.FriendlyName,
			MatchPattern: p.MatchPattern,
			IsRunning:    p.IsRunning,
			PID:          p.PID,
			CPUPercent:   p.CPUPercent,
			MemPercent:   p.MemPercent,
			Cmdline:      p.Cmdline,
		}
	}

	payload := models.CheckInRequest{
		Hostname:      hostname,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		ClientVersion: version.Version,
		ClientID:      clientID,
		Metrics: models.MetricsPayload{
			CPUPercent:     metrics.CPUPercent,
			MemPercent:     metrics.MemPercent,
			MemTotalBytes:  metrics.MemTotal,
			MemUsedBytes:   metrics.MemUsed,
			DiskPercent:    metrics.DiskPercent,
			DiskTotalBytes: metrics.DiskTotal,
			DiskUsedBytes:  metrics.DiskUsed,
		},
		Processes: processes,
	}

	for _, c := range checks {
		payload.Checks = append(payload.Checks, models.CheckPayload{
			FriendlyName: c.FriendlyName,
			CheckType:    c.CheckType,
			Healthy:      c.Healthy,
			Message:      c.Message,
			State:        c.State,
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", r.serverURL+"/api/v1/checkin", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Password", r.password)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send check-in: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication failed: check your password")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var result models.CheckInResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}
