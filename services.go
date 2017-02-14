package main

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"

	"github.com/WiseGrowth/wisebot-operator/command"
	"github.com/WiseGrowth/wisebot-operator/git"
)

// Service encapsulates a command an its repository
type Service struct {
	Name string
	cmd  *command.Command
	repo *git.Repo
}

func newService(name string, c *command.Command, r *git.Repo) *Service {
	c.Updater = r
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
	return s.cmd.Update()
}

// ServiceStore represents a set of commands.
// It has convinient methods to run and stop all
// commands.
type ServiceStore map[string]*Service

// MarshalJSON implements json marshal interface
func (ss ServiceStore) MarshalJSON() ([]byte, error) {
	services := make([]*Service, len(ss))
	for _, svc := range ss {
		services = append(services, svc)
	}

	payload := struct {
		Data []*Service `json:"data"`
	}{services}
	return json.Marshal(payload)
}

func (s *Service) logger() *log.Entry {
	return log.WithFields(log.Fields{
		"name":         s.Name,
		"version":      s.cmd.Version,
		"status":       s.cmd.Status(),
		"repo_version": s.repo.CurrentHead(),
	})
}

// Update search the given command in the map and runs its
// Update function. If the command is not found, an error is
// returned.
func (ss *ServiceStore) Update(name string) error {
	svc, ok := (*ss)[name]

	if !ok {
		return fmt.Errorf("services: service with name %q not found", name)
	}

	cmd := svc.cmd

	svc.logger().Info("Running update")
	cmd.SetStatus(command.StatusUpdating)
	updated, err := cmd.Update()
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
	(*ss)[name] = newService(svc.Name, cmd, svc.repo)

	svc.logger().Info("Starting updated service")
	if err := cmd.Start(); err != nil {
		return err
	}

	return nil
}

// Add builds and add the service to the list
func (ss *ServiceStore) Add(name string, c *command.Command, r *git.Repo) {
	s := newService(name, c, r)
	(*ss)[s.Name] = s
}

// StartService starts a specific service inside the store.
// If the service is not found in the list, it returns an error.
func (ss *ServiceStore) StartService(name string) error {
	svc, ok := (*ss)[name]
	if !ok {
		return fmt.Errorf("services: service %q not found for starting", name)
	}

	svc.logger().Info("Starting")
	return svc.cmd.Start()
}

// Start starts all the commands inside the command list by
// looping and calling each command Start function.
func (ss *ServiceStore) Start() error {
	for _, svc := range *ss {
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
	svc, ok := (*ss)[name]
	if !ok {
		return fmt.Errorf("services: service %q not found for stopping", name)
	}

	svc.logger().Info("Stopping")
	return svc.cmd.Stop()
}

// Stop stops all the services inside the store by
// looping and calling each service command's Stop function.
func (ss *ServiceStore) Stop() error {
	for _, svc := range *ss {
		svc.logger().Info("Stopping")
		if err := svc.cmd.Stop(); err != nil {
			return err
		}
	}

	return nil
}
