package systemd

import (
	"bytes"
	"fmt"
	"os/exec"
)

// ServiceStatus represents a systemd service status
type ServiceStatus string

// Systemd Service Statuses
const (
	ServiceStatusIdle     ServiceStatus = "idle"
	ServiceStatusInactive ServiceStatus = "inactive"
	ServiceStatusRunning  ServiceStatus = "running"
	ServiceStatusError    ServiceStatus = "error"
)

// Exists checks with systemd if the given service exists.
func Exists(name string) bool {
	stdout := &bytes.Buffer{}
	status := exec.Command("systemctl", "status", name)
	status.Stdout = stdout

	status.Run()

	if bytes.Contains(stdout.Bytes(), []byte(`Loaded: not-found`)) {
		return false
	}

	return true
}

// Status returns service status by asking systemd.
func Status(name string) (ServiceStatus, error) {
	stdout := &bytes.Buffer{}

	isActive := exec.Command("systemctl", "is-active", name)
	isActive.Stdout = stdout

	err := isActive.Run()
	if err == nil {
		return ServiceStatusRunning, nil
	}

	if _, ok := err.(*exec.ExitError); !ok {
		return ServiceStatusError, err
	}

	out := string(bytes.TrimSpace(stdout.Bytes()))
	switch out {
	case "inactive":
		return ServiceStatusInactive, nil
	case "failed":
		return ServiceStatusError, nil
	case "activating":
		return ServiceStatusRunning, nil
	case "unknown":
		return ServiceStatusIdle, nil
	default:
		return "", fmt.Errorf("unknown status received from systemd: %q", out)
	}
}

// Start starts the service by telling systemd to start it.
func Start(name string) error {
	return exec.Command("sudo", "systemctl", "start", name).Run()
}

// Restart restarts the service by telling systemd to restart it.
func Restart(name string) error {
	return exec.Command("sudo", "systemctl", "restart", name).Run()
}

// Stop stops the service by telling systemd to stop it.
func Stop(name string) error {
	return exec.Command("sudo", "systemctl", "stop", name).Run()
}
