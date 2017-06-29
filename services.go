package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/WiseGrowth/go-wisebot/led"
	"github.com/WiseGrowth/go-wisebot/logger"
	"github.com/WiseGrowth/wisebot-operator/command"
	"github.com/WiseGrowth/wisebot-operator/git"
)

const (
	maxRetries = 3
)

// Service encapsulates a command an its repository
type Service struct {
	Name     string
	cmd      *command.Command
	repo     *git.Repo
	finished chan error    // command error
	stop     chan struct{} // stop command watcher

	sync.Mutex // guards Update and Bootstrap functions.
}

func newService(name string, c *command.Command, r *git.Repo) *Service {
	svc := &Service{Name: name, cmd: c, repo: r}
	svc.finished = make(chan error, 1)
	c.Finish = svc.finished

	return svc
}

// MarshalJSON implements json marshal interface
func (s *Service) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name        string         `json:"name"`
		Version     string         `json:"version"`
		Status      command.Status `json:"status"`
		RepoVersion string         `json:"repo_version"`
	}{
		Name:        s.Name,
		Version:     s.cmd.Version,
		Status:      s.cmd.Status(),
		RepoVersion: s.repo.CurrentHead(),
	})
}

// Start runs the service observe function in background and then proxies the
// Start function call to its internal command struct.
func (s *Service) Start() error {
	go s.observe()
	return s.cmd.Start()
}

// Stop proxies function to the its command.
func (s *Service) Stop() error {
	return s.cmd.Stop()
}

// Update proxies function to the its command.
func (s *Service) Update() (bool, error) {
	s.Lock()
	defer s.Unlock()

	s.cmd.SetStatus(command.StatusUpdating)
	return s.cmd.Update(s.repo)
}

// Bootstrap proxies function to the its repo.
func (s *Service) Bootstrap(update bool) error {
	s.Lock()
	defer s.Unlock()

	if err := s.repo.Bootstrap(update); err != nil {
		return err
	}

	s.cmd.Version = s.repo.CurrentHead()

	return nil
}

// observe observes if the internal service command exited with error or not.
// If the command exited with error, it notifies the led service.
func (s *Service) observe() {
	log := s.logger()
	log.Info("Start observing")
	running := true
	for running {
		select {
		case err := <-s.finished:
			if err != nil {
				go notifyServiceExitErrorWithRetry(s)
			}
			running = false
		}
	}
	log.Info("Stop observing")
}

// notifyInternetWithRetry calls led.PostServiceExitError until exits without
// error. The retry interval is 3 seconds.
func notifyServiceExitErrorWithRetry(s *Service) {
	now := time.Now()
	log := s.logger()
	try := 0
	for {
		if try == maxRetries {
			log.Debug("max service exited error post retries reached")
			break
		}

		if err := led.PostServiceExitError(s.Name, now); err != nil {
			log.Error(err)
			time.Sleep(3 * time.Second)
			log.Debug("service exited error post failed, retrying")
			try++
			continue
		}

		log.Debug("service exited error posted!")
		break
	}
}

// ServiceStore represents a set of commands. It has convinient methods to run
// and stop all commands.
type ServiceStore struct {
	mu   sync.RWMutex
	list map[string]*Service
}

// MarshalJSON implements json marshal interface
func (ss *ServiceStore) MarshalJSON() ([]byte, error) {
	svcs := make([]*Service, len(ss.list))

	i := 0
	for _, svc := range ss.list {
		ss.mu.RLock()
		svcs[i] = svc
		i++
		ss.mu.RUnlock()
	}

	return json.Marshal(svcs)
}

func (s *Service) logger() *logrus.Entry {
	return logger.GetLogger().WithFields(logrus.Fields{
		"name":            s.Name,
		"command_version": s.cmd.Version,
		"status":          s.cmd.Status(),
		"repo_version":    s.repo.CurrentHead(),
	})
}

// Find looks the service in the list by its name. If the service does not
// exists, it returns a nil Service and a false value.
func (ss *ServiceStore) Find(name string) (svc *Service, ok bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	svc, ok = ss.list[name]
	return svc, ok
}

// Bootstrap loops each service in the list and calls the bootstrap function.
func (ss *ServiceStore) Bootstrap(update bool) error {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	for _, svc := range ss.list {
		if err := svc.Bootstrap(update); err != nil {
			return err
		}
	}

	return nil
}

// Update search the given command in the map and runs its Update function. If
// the command is not found, an error is returned.
func (ss *ServiceStore) Update(name string) error {
	svc, ok := ss.Find(name)

	if !ok {
		return fmt.Errorf("services: service with name %q not found", name)
	}

	cmd := svc.cmd

	svc.logger().Info("Running update")
	oldStatus := svc.cmd.Status()
	updated, err := svc.Update()
	if err != nil {
		svc.cmd.SetStatus(oldStatus)
		return err
	}

	if !updated {
		svc.logger().Info("No new updates")
		svc.cmd.SetStatus(oldStatus)
		return nil
	}

	svc.logger().Info("Update found, stopping")
	if err := cmd.Stop(); err != nil {
		return err
	}

	cmd = cmd.Clone()
	ss.Save(svc.Name, cmd, svc.repo)

	svc.logger().Info("Starting updated service")
	if err := cmd.Start(); err != nil {
		return err
	}

	return nil
}

// Save builds and add the service to the list.
func (ss *ServiceStore) Save(name string, c *command.Command, r *git.Repo) *Service {
	s := newService(name, c, r)

	ss.mu.RLock()
	if ss.list == nil {
		ss.list = make(map[string]*Service)
	}
	ss.mu.RUnlock()

	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.list[s.Name] = s

	return s
}

// StartService starts a specific service inside the store. If the service is
// not found in the list, it returns an error.
func (ss *ServiceStore) StartService(name string) error {
	svc, ok := ss.Find(name)
	if !ok {
		return fmt.Errorf("services: service %q not found for starting", name)
	}

	status := svc.cmd.Status()
	if status == command.StatusCrashed || status == command.StatusBootingError || status == command.StatusStopped || status == command.StatusDone {
		newCmd := svc.cmd.Clone()
		svc = ss.Save(svc.Name, newCmd, svc.repo)
	}

	svc.logger().Info("Starting")
	return svc.Start()
}

// Start starts all the commands inside the command list by looping and calling
// each command Start function.
func (ss *ServiceStore) Start() error {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	for _, svc := range ss.list {
		svc.logger().Info("Starting")
		if err := svc.Start(); err != nil {
			return err
		}
	}

	return nil
}

// StopService stops a specific service inside the store. If the service is not
// found in the list, it returns an error.
func (ss *ServiceStore) StopService(name string) error {
	svc, ok := ss.Find(name)
	if !ok {
		return fmt.Errorf("services: service %q not found for stopping", name)
	}

	svc.logger().Info("Stopping")
	defer svc.logger().Info("Stopped")
	return svc.Stop()
}

// Stop stops all the services inside the store by looping and calling each
// service command's Stop function.
func (ss *ServiceStore) Stop() error {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	for _, svc := range ss.list {
		svc.logger().Info("Stopping")
		if err := svc.Stop(); err != nil {
			return err
		}
	}

	return nil
}
