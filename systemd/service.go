package systemd

import (
	"fmt"
	"os/exec"
)

// ServiceStatus represents a systemd service status
type ServiceStatus string

// Systemd Service Statuses
const (
	ServiceStatusIdle     ServiceStatus = "idle"
	ServiceStatusRunning  ServiceStatus = "running"
	ServiceStatusError    ServiceStatus = "error"
	ServiceStatusUpdating ServiceStatus = "updating"
	ServiceStatusDone     ServiceStatus = "succeed"
	ServiceStatusStopped  ServiceStatus = "stopped"
)

// Exists checks with systemd if the given service exists.
func Exists(name string) bool {
	// TODO: implement
	return false
}

// Status returns service status by asking systemd.
func Status(name string) (ServiceStatus, error) {
	out, err := exec.Command("sudo", "systemctl", "status", name).Output()
	if err != nil {
		return "", err
	}

	// TODO: parse output and check the service status
	fmt.Println(string(out))

	return ServiceStatusIdle, nil
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
