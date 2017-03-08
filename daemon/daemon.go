package daemon

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/WiseGrowth/wisebot-operator/git"
	"github.com/WiseGrowth/wisebot-operator/logger"
	"github.com/WiseGrowth/wisebot-operator/systemd"
)

// errors
var (
	ErrSystemdServiceNotExists = errors.New("systemd service doesn't exists")
)

// Daemon encapsulates a command an its repository
type Daemon struct {
	Name string
	repo *git.Repo

	mu       sync.RWMutex
	updating bool
}

// NewDaemon initializes a a daemon but it returns an error if the
// systemd.Service does not exists.
func NewDaemon(name string, r *git.Repo) (*Daemon, error) {
	exists := systemd.Exists(name)
	if !exists {
		return nil, ErrSystemdServiceNotExists
	}

	return &Daemon{Name: name, repo: r}, nil
}

// MarshalJSON implements json marshal interface
func (d *Daemon) MarshalJSON() (bytes []byte, err error) {
	var status systemd.ServiceStatus

	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.updating {
		status = systemd.ServiceStatusUpdating
	} else {
		if status, err = systemd.Status(d.Name); err != nil {
			return nil, err
		}
	}

	return json.Marshal(struct {
		Name        string                `json:"name"`
		Status      systemd.ServiceStatus `json:"status"`
		RepoVersion string                `json:"repo_version"`
	}{
		Name:        d.Name,
		Status:      status,
		RepoVersion: d.repo.CurrentHead(),
	})
}

// Update calls Daemon repo Update function if exists.
func (d *Daemon) Update() (updated bool, err error) {
	if d.repo == nil {
		return true, nil
	}

	d.mu.Lock()
	d.updating = true
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		d.updating = false
		d.mu.Unlock()
	}()

	oldSha := d.repo.CurrentHead()
	newSha, err := d.repo.Update()
	if err != nil {
		return false, err
	}

	return oldSha != newSha, nil
}

// Bootstrap proxies function to the its repo if exists.
func (d *Daemon) Bootstrap(update bool) error {
	if d.repo == nil {
		return nil
	}

	if err := d.repo.Bootstrap(update); err != nil {
		return err
	}

	return nil
}

func (d *Daemon) logger() *logrus.Entry {
	status, _ := systemd.Status(d.Name)
	return logger.GetLogger().WithFields(logrus.Fields{
		"name":         d.Name,
		"status":       status,
		"repo_version": d.repo.CurrentHead(),
	})
}
