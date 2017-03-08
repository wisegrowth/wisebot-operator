package main

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

func newDaemon(name string, r *git.Repo) (*Daemon, error) {
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

// DaemonStore represents a set of daemons.
// It has convinient methods to run and stop all
// commands.
type DaemonStore struct {
	mu   sync.RWMutex
	list map[string]*Daemon
}

// MarshalJSON implements json marshal interface
func (ds *DaemonStore) MarshalJSON() ([]byte, error) {
	svcs := make([]*Daemon, len(ds.list))

	i := 0
	for _, svc := range ds.list {
		ds.mu.RLock()
		svcs[i] = svc
		i++
		ds.mu.RUnlock()
	}

	return json.Marshal(svcs)
}

func (d *Daemon) logger() *logrus.Entry {
	status, _ := systemd.Status(d.Name)
	return logger.GetLogger().WithFields(logrus.Fields{
		"name":         d.Name,
		"status":       status,
		"repo_version": d.repo.CurrentHead(),
	})
}

// Find looks the service in the list by its name. If the service does not
// exists, it returns a nil Service and a false value.
func (ds *DaemonStore) Find(name string) (daemon *Daemon, ok bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	daemon, ok = ds.list[name]
	return daemon, ok
}

// Bootstrap loops each service in the list and calls the bootstrap function.
func (ds *DaemonStore) Bootstrap(update bool) error {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	for _, svc := range ds.list {
		if err := svc.Bootstrap(update); err != nil {
			return err
		}
	}

	return nil
}

// Update search the given command in the map and runs its
// Update function. If the command is not found, an error is
// returned.
func (ds *DaemonStore) Update(name string) error {
	daemon, ok := ds.Find(name)

	if !ok {
		return fmt.Errorf("daemons: daemon with name %q not found", name)
	}

	daemon.logger().Info("Running update")
	updated, err := daemon.Update()
	if err != nil {
		return err
	}

	if !updated {
		daemon.logger().Info("No new updates")
		return nil
	}

	daemon.logger().Info("Update found, restarting updated daemon")
	return systemd.Restart(daemon.Name)
}

// Save initialize the list and add the daemon to it.
func (ds *DaemonStore) Save(d *Daemon) *Daemon {
	ds.mu.RLock()
	if ds.list == nil {
		ds.list = make(map[string]*Daemon)
	}
	ds.mu.RUnlock()

	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.list[d.Name] = d

	return d
}

// StartDaemon starts a specific daemon inside the store.
// If the daemon is not found in the list, it returns an error.
func (ds *DaemonStore) StartDaemon(name string) error {
	d, ok := ds.Find(name)
	if !ok {
		return fmt.Errorf("daemons: daemon %q not found for starting", name)
	}

	d.logger().Info("Starting")
	return systemd.Start(d.Name)
}

// StopDaemon stops a specific daemon inside the store.
// If the daemon is not found in the list, it returns an error.
func (ds *DaemonStore) StopDaemon(name string) error {
	d, ok := ds.Find(name)
	if !ok {
		return fmt.Errorf("daemons: daemon %q not found for stopping", name)
	}

	d.logger().Info("Stopping")
	defer d.logger().Info("Stopped")

	return systemd.Stop(name)
}
