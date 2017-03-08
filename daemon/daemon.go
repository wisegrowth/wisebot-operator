package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/WiseGrowth/wisebot-operator/git"
	"github.com/WiseGrowth/wisebot-operator/logger"
	"github.com/WiseGrowth/wisebot-operator/systemd"
)

// Status represents daemon status
type Status string

// Daemon possible statuses
const (
	StatusError    = "error"
	StatusRunning  = "running"
	StatusUpdating = "updating"
	StatusStopped  = "stopped"
)

// errors
var (
	ErrSystemdServiceNotExists = errors.New("systemd service doesn't exists")
)

// Daemon encapsulates a command an its repository
type Daemon interface {
	Name() string
	Start() error
	Restart() error
	Stop() error
	Status() (Status, error)
	// Update updates daemon codebase and returns a boolean indicating if there is
	// new code or not.
	Update() (bool, error)
	// Bootstrap pulls the daemon codebase if does not exists. If the codebase
	// exists, depending on the given update parameter, updates the codebase.
	Bootstrap(update bool) error
	// Logger returns an initialized logger that contains daemon specific info.
	Logger() *logrus.Entry
}

// daemon encapsulates a command an its repository
type daemon struct {
	name string
	cu   codebaseUpdater

	mu       sync.RWMutex
	updating bool
}

type codebaseUpdater interface {
	Bootstrap(bool) error
	CurrentHead() string
	Update() (string, error)
}

// NewDaemon initializes a a daemon but it returns an error if the
// systemd.Service does not exists.
func NewDaemon(name string, r *git.Repo) (Daemon, error) {
	exists := systemd.Exists(name)
	if !exists {
		return nil, ErrSystemdServiceNotExists
	}

	return &daemon{name: name, cu: r}, nil
}

// MarshalJSON implements json marshal interface
func (d *daemon) MarshalJSON() (bytes []byte, err error) {
	status, _ := d.Status()

	return json.Marshal(struct {
		Name        string `json:"name"`
		Status      Status `json:"status"`
		RepoVersion string `json:"repo_version"`
	}{
		Name:        d.name,
		Status:      status,
		RepoVersion: d.cu.CurrentHead(),
	})
}

func (d *daemon) Name() string {
	return d.name
}

// Start uses systemd to start the daemon service.
func (d *daemon) Start() error {
	return systemd.Start(d.name)
}

// Restart uses systemd to restart the daemon service.
func (d *daemon) Restart() error {
	return systemd.Restart(d.name)
}

// Stop uses systemd to stop the daemon service.
func (d *daemon) Stop() error {
	return systemd.Stop(d.name)
}

// Update calls Daemon updater Update function if exists.
func (d *daemon) Update() (updated bool, err error) {
	if d.cu == nil {
		return true, nil
	}

	defer func() {
		d.mu.Lock()
		d.updating = false
		d.mu.Unlock()
	}()

	d.mu.Lock()
	d.updating = true
	d.mu.Unlock()

	oldSha := d.cu.CurrentHead()
	newSha, err := d.cu.Update()
	if err != nil {
		return false, err
	}

	return oldSha != newSha, nil
}

func (d *daemon) Status() (Status, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.updating {
		return StatusUpdating, nil
	}

	systemdStatus, err := systemd.Status(d.name)
	if err != nil {
		return "", err
	}

	switch systemdStatus {
	case systemd.ServiceStatusIdle:
		return StatusStopped, nil
	case systemd.ServiceStatusRunning:
		return StatusRunning, nil
	case systemd.ServiceStatusError:
		return StatusError, nil
	default:
		return "", fmt.Errorf("unknown status received from systemd %q", systemdStatus)
	}
}

// Bootstrap proxies function to the its updater if exists.
func (d *daemon) Bootstrap(update bool) error {
	if d.cu == nil {
		return nil
	}

	if err := d.cu.Bootstrap(update); err != nil {
		return err
	}

	return nil
}

func (d *daemon) Logger() *logrus.Entry {
	status, _ := systemd.Status(d.name)
	return logger.GetLogger().WithFields(logrus.Fields{
		"name":         d.name,
		"status":       status,
		"repo_version": d.cu.CurrentHead(),
	})
}
