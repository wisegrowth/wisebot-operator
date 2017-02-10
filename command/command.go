package command

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	log "github.com/Sirupsen/logrus"
)

const (
	statusIdle     = "idle"
	statusRunning  = "running"
	statusError    = "error"
	statusUpdating = "updating"
	statusDone     = "succeed"
	statusStopped  = "stopped"
)

// Command represents a os level command, which can also
// receive a logger file in order to dump the output to it.
type Command struct {
	Log io.WriteCloser
	Cmd *exec.Cmd

	// stderr *bytes.Buffer // Maybe this will help us debugging when a command fails?

	Slug    string
	Version string

	status string

	Updater Updater

	cmdName string
	cmdArgs []string
}

// Updater knows how to update the codebase of a specific
// command codebase.
type Updater interface {
	Update() (newVersion string, err error)
}

// MarshalJSON implements the json interface
func (c *Command) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Slug    string `json:"slug"`
		Version string `json:"version"`
		Status  string `json:"status"`
	}{
		Slug:    c.Slug,
		Status:  c.Status(),
		Version: c.Version,
	})
}

// Update uses the updater in order to update the code base
// and the command version.
// If no updater is found, it returns an error.
// Update function returns a boolean that indicates
// if the code was either updated or not. Knowing if
// the command was updated is important in order to
// decide if we need to restart it or not.
func (c *Command) Update() (updated bool, err error) {
	if c.Updater == nil {
		return false, fmt.Errorf("command: no updater for %q command", c.Slug)
	}

	oldVersion := c.Version
	newVersion, err := c.Updater.Update()
	if err != nil {
		return false, err
	}

	if newVersion != oldVersion {
		c.Version = newVersion
		updated = true
	}

	c.logger().Debugf("updated: %v - oldVersion: %q - newVersion: %q", updated, oldVersion, newVersion)

	return updated, nil
}

// Status check the command's process state and returns
// a verbose status.
func (c *Command) Status() string {
	if c.Cmd.ProcessState == nil {
		return c.status
	}

	ps := c.Cmd.ProcessState

	if ps.Success() {
		return statusDone
	}

	if ps.Exited() {
		return statusError
	}

	return c.status
}

// CloseLog safely close the command's logger.
// If the logger is just os.Stdout, it does not
// close it.
func (c *Command) CloseLog() error {
	if c.Log == nil || c.Log == os.Stdout {
		return nil
	}

	return c.Log.Close()
}

// Stop stops the command and closes the log file
// if exists.
func (c *Command) Stop() error {
	if c.Log != nil {
		defer c.CloseLog()
	}

	if c.status == statusStopped {
		return fmt.Errorf("commands: command %q is already stopped", c.Slug)
	}

	c.status = statusStopped

	c.logger().Info("Killing process")
	if c.Cmd.Process == nil {
		return nil
	}

	// the ProcessState only exists if either the process exited,
	// or we called Run or Wait functions.
	ps := c.Cmd.ProcessState
	if ps != nil && ps.Exited() {
		return nil
	}

	if err := c.Cmd.Process.Signal(os.Interrupt); err != nil {
		return err
	}

	_, err := c.Cmd.Process.Wait()
	return err
}

// Wait only proxies the function call to the
// os.Command.Wait function.
func (c *Command) Wait() error {
	return c.Cmd.Wait()
}

// Start starts the process and pipes the command's
// output to the log file. If at any point there is an error
// it also closes the file if exists.
func (c *Command) Start() error {
	if c.Log == os.Stdout || c.Log == nil {
		c.Cmd.Stdout = c.Log
	} else {
		out, err := c.Cmd.StdoutPipe()
		if err != nil {
			c.CloseLog()
			return err
		}

		go func() {
			for {
				r := bufio.NewReader(out)
				l, _, err := r.ReadLine()
				if err != nil {
					if err != io.EOF {
						c.CloseLog()
						panic(err)
					}
				}

				c.Log.Write(l)
				c.Log.Write([]byte("\n"))
			}
		}()
	}

	c.logger().Info("Starting")
	if err := c.Cmd.Start(); err != nil {
		c.CloseLog()
		return err
	}

	c.status = statusRunning

	return nil
}

func (c *Command) logger() *log.Entry {
	return log.WithFields(log.Fields{
		"command": c.Slug,
		"version": c.Version,
	})
}

// Success just proxies the function call to the
// command.ProcessState struct.
func (c *Command) Success() bool {
	return c.Cmd.ProcessState.Success()
}

// Commands represents a set of commands.
// It has convinient methods to run and stop all
// commands.
type Commands map[string]*Command

// Update search the given command in the map and runs its
// Update function. If the command is not found, an error is
// returned.
func (c *Commands) Update(cmdSlug string) error {
	cmd, ok := (*c)[cmdSlug]

	if !ok {
		return fmt.Errorf("commands: command with slug %q not found", cmdSlug)
	}

	cmd.logger().Info("Running update")
	cmd.status = statusUpdating
	updated, err := cmd.Update()
	if err != nil {
		cmd.status = statusRunning
		return err
	}

	if !updated {
		return nil
	}

	if err := cmd.Stop(); err != nil {
		return err
	}

	updater := cmd.Updater
	cmd = NewCommand(cmd.Log, cmd.Slug, cmd.Version, cmd.cmdName, cmd.cmdArgs...)
	cmd.Updater = updater
	(*c)[cmdSlug] = cmd

	if err := cmd.Start(); err != nil {
		return err
	}

	return nil
}

// Add adds the received command into the commands list
func (c *Commands) Add(cmd *Command) {
	(*c)[cmd.Slug] = cmd
}

// StartCommand starts a specific command inside the command list.
// If the command is not found in the list, it returns an error.
func (c *Commands) StartCommand(slug string) error {
	cmd, ok := (*c)[slug]
	if !ok {
		return fmt.Errorf("command: command %q not found for starting", slug)
	}

	return cmd.Start()
}

// Start starts all the commands inside the command list by
// looping and calling each command Start function.
func (c *Commands) Start() error {
	for _, cmd := range *c {
		if err := cmd.Start(); err != nil {
			return err
		}
	}

	return nil
}

// StopCommand stops a specific command inside the command list.
// If the command is not found in the list, it returns an error.
func (c *Commands) StopCommand(slug string) error {
	cmd, ok := (*c)[slug]
	if !ok {
		return fmt.Errorf("command: command %q not found for stopping", slug)
	}

	return cmd.Stop()
}

// Stop stops all the commands inside the command list by
// looping and calling each command Stop function.
func (c *Commands) Stop() error {
	for _, cmd := range *c {
		if err := cmd.Stop(); err != nil {
			return err
		}
	}

	return nil
}

// NewCommand returns an initalized command pointer.
func NewCommand(log io.WriteCloser, slug, version, name string, args ...string) *Command {
	cmd := &Command{
		Log:     log,
		Cmd:     exec.Command(name, args...),
		Slug:    slug,
		Version: version,

		status:  statusIdle,
		cmdName: name,
		cmdArgs: args,
	}

	cmd.Cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	cmd.Updater = &noopUpdater{cmd}

	return cmd
}

type noopUpdater struct {
	*Command
}

func (nu *noopUpdater) Update() (string, error) {
	nu.logger().Warn("Noop Updater Called")

	return nu.Version, nil
}
