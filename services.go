package main

import (
	"encoding/json"
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"

	"github.com/WiseGrowth/wisebot-operator/command"
	"github.com/WiseGrowth/wisebot-operator/git"
	"github.com/WiseGrowth/wisebot-operator/logger"
)

// Service encapsulates a command an its repository
type Service struct {
	Name string
	cmd  *command.Command
	repo *git.Repo
}

func newService(name string, c *command.Command, r *git.Repo) *Service {
	return &Service{Name: name, cmd: c, repo: r}
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

// Update proxies function to the its command
func (s *Service) Update() (bool, error) {
	return s.cmd.Update(s.repo)
}

// ServiceStore represents a set of commands.
// It has convinient methods to run and stop all
// commands.
type ServiceStore struct {
	mu   sync.RWMutex
	list map[string]*Service
}

// MarshalJSON implements json marshal interface
func (ss *ServiceStore) MarshalJSON() ([]byte, error) {
	services := make([]*Service, len(ss.list))
	for _, svc := range ss.list {
		ss.mu.RLock()
		services = append(services, svc)
		ss.mu.RUnlock()
	}

	payload := struct {
		Data []*Service `json:"data"`
	}{services}
	return json.Marshal(payload)
}

func (s *Service) logger() *log.Entry {
	return logger.GetLogger().WithFields(log.Fields{
		"name":         s.Name,
		"version":      s.cmd.Version,
		"status":       s.cmd.Status(),
		"repo_version": s.repo.CurrentHead(),
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

// Update search the given command in the map and runs its
// Update function. If the command is not found, an error is
// returned.
func (ss *ServiceStore) Update(name string) error {
	svc, ok := ss.Find(name)

	if !ok {
		return fmt.Errorf("services: service with name %q not found", name)
	}

	cmd := svc.cmd

	svc.logger().Info("Running update")
	cmd.SetStatus(command.StatusUpdating)
	updated, err := svc.Update()
	if err != nil {
		svc.cmd.SetStatus(command.StatusRunning)
		return err
	}

	if !updated {
		svc.logger().Info("No new updates")
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
func (ss *ServiceStore) Save(name string, c *command.Command, r *git.Repo) {
	s := newService(name, c, r)

	ss.mu.RLock()
	if ss.list == nil {
		ss.list = make(map[string]*Service)
	}
	ss.mu.RUnlock()

	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.list[s.Name] = s
}

// StartService starts a specific service inside the store.
// If the service is not found in the list, it returns an error.
func (ss *ServiceStore) StartService(name string) error {
	svc, ok := ss.Find(name)
	if !ok {
		return fmt.Errorf("services: service %q not found for starting", name)
	}

	svc.logger().Info("Starting")
	return svc.cmd.Start()
}

// Start starts all the commands inside the command list by
// looping and calling each command Start function.
func (ss *ServiceStore) Start() error {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	for _, svc := range ss.list {
		svc.logger().Info("Starting")
		if err := svc.cmd.Start(); err != nil {
			return err
		}
	}

	return nil
}

// StopService stops a specific service inside the store.
// If the service is not found in the list, it returns an error.
func (ss *ServiceStore) StopService(name string) error {
	svc, ok := ss.Find(name)
	if !ok {
		return fmt.Errorf("services: service %q not found for stopping", name)
	}

	svc.logger().Info("Stopping")
	return svc.cmd.Stop()
}

// Stop stops all the services inside the store by
// looping and calling each service command's Stop function.
func (ss *ServiceStore) Stop() error {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	for _, svc := range ss.list {
		svc.logger().Info("Stopping")
		if err := svc.cmd.Stop(); err != nil {
			return err
		}
	}

	return nil
}
