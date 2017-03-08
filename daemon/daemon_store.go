package daemon

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Store represents a set of daemons.
// It has convinient methods to run and stop all
// commands.
type Store struct {
	mu   sync.RWMutex
	list map[string]Daemon
}

// MarshalJSON implements json marshal interface
func (s *Store) MarshalJSON() ([]byte, error) {
	svcs := make([]Daemon, len(s.list))

	i := 0
	for _, svc := range s.list {
		s.mu.RLock()
		svcs[i] = svc
		i++
		s.mu.RUnlock()
	}

	return json.Marshal(svcs)
}

// Find looks the service in the list by its name. If the service does not
// exists, it returns a nil Service and a false value.
func (s *Store) Find(name string) (daemon Daemon, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	daemon, ok = s.list[name]
	return daemon, ok
}

// Bootstrap loops each service in the list and calls the bootstrap function.
func (s *Store) Bootstrap(update bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, svc := range s.list {
		if err := svc.Bootstrap(update); err != nil {
			return err
		}
	}

	return nil
}

// Update search the given command in the map and runs its Update function. If
// the command is not found, an error is returned.
func (s *Store) Update(name string) error {
	daemon, ok := s.Find(name)

	if !ok {
		return fmt.Errorf("daemons: daemon with name %q not found", name)
	}

	daemon.Logger().Info("Running update")
	updated, err := daemon.Update()
	if err != nil {
		return err
	}

	if !updated {
		daemon.Logger().Info("No new updates")
		return nil
	}

	daemon.Logger().Info("Update found, restarting updated daemon")
	return daemon.Restart()
}

// Save initialize the list and add the daemon to it.
func (s *Store) Save(d Daemon) Daemon {
	s.mu.RLock()
	if s.list == nil {
		s.list = make(map[string]Daemon)
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.list[d.Name()] = d

	return d
}

// StartDaemon starts a specific daemon inside the store.
// If the daemon is not found in the list, it returns an error.
func (s *Store) StartDaemon(name string) error {
	d, ok := s.Find(name)
	if !ok {
		return fmt.Errorf("daemons: daemon %q not found for starting", name)
	}

	d.Logger().Info("Starting")
	return d.Start()
}

// StopDaemon stops a specific daemon inside the store.
// If the daemon is not found in the list, it returns an error.
func (s *Store) StopDaemon(name string) error {
	d, ok := s.Find(name)
	if !ok {
		return fmt.Errorf("daemons: daemon %q not found for stopping", name)
	}

	d.Logger().Info("Stopping")
	defer d.Logger().Info("Stopped")

	return d.Stop()
}
