package command

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Status represents the current command status
type Status string

// Command statuses
const (
	StatusIdle     Status = "idle"
	StatusRunning  Status = "running"
	StatusError    Status = "error"
	StatusUpdating Status = "updating"
	StatusDone     Status = "succeed"
	StatusStopped  Status = "stopped"
)

// Command represents a os level command, which can also receive a logger file
// in order to dump the output to it.
type Command struct {
	Log io.WriteCloser
	Cmd *exec.Cmd

	Version string
	Updater Updater

	// Maybe this will help us debugging when a command fails?
	// stderr *bytes.Buffer

	status   Status
	execName string
	execArgs []string
}

// Clone clones the command by instantiate a new one with same attributes
// and returns it. This is handy if you need to restart the process, first
// you stop it, then clone it, then you start the new cloned process.
func (c *Command) Clone() *Command {
	cmd := NewCommand(c.Log, c.Version, c.execName, c.execArgs...)
	cmd.Updater = c.Updater
	return cmd
}

// Updater knows how to update the codebase of a specific command codebase.
type Updater interface {
	Update() (newVersion string, err error)
}

// MarshalJSON implements the json interface
func (c *Command) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Slug    string `json:"slug"`
		Version string `json:"version"`
		Status  Status `json:"status"`
	}{
		Slug:    c.Slug(),
		Status:  c.Status(),
		Version: c.Version,
	})
}

// Slug combines the command execName and execArgs in order to return a verbose
// identifier.
func (c *Command) Slug() string {
	return fmt.Sprintf("%s %s", c.execName, strings.Join(c.execArgs, " "))
}

// SetStatus sets the command current status
func (c *Command) SetStatus(status Status) {
	c.status = status
}

// Update uses the updater in order to update the code base and the command
// version.  If no updater is found, it returns an error. Update function
// returns a boolean that indicate if the code was either updated or not.
// Knowing if the command was updated is important in order to decide if we
// need to restart it or not.
func (c *Command) Update() (updated bool, err error) {
	if c.Updater == nil {
		return false, fmt.Errorf("command: no updater for %q command", c.Slug())
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

	return updated, nil
}

// Status check the command's process state and returns a verbose status.
func (c *Command) Status() Status {
	if c.Cmd.ProcessState == nil {
		return c.status
	}

	ps := c.Cmd.ProcessState

	if ps.Success() {
		return StatusDone
	}

	if ps.Exited() {
		return StatusError
	}

	return c.status
}

// CloseLog safely close the command's logger. If the logger is just os.Stdout,
// it does not close it.
func (c *Command) CloseLog() error {
	if c.Log == nil || c.Log == os.Stdout {
		return nil
	}

	return c.Log.Close()
}

// Stop stops the command and closes the log file if exists.
func (c *Command) Stop() error {
	if c.Log != nil {
		defer c.CloseLog()
	}

	if c.status == StatusStopped {
		return fmt.Errorf("commands: command %q is already stopped", c.Slug())
	}

	c.status = StatusStopped

	if c.Cmd.Process == nil {
		return nil
	}

	// the ProcessState only exists if either the process exited, or we called
	// Run or Wait functions.
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

// Wait only proxies the function call to the  os.Command.Wait function.
func (c *Command) Wait() error {
	return c.Cmd.Wait()
}

// Start starts the process and pipes the command's output to the log file.
// If at any point there is an error it also closes the file if exists.
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

	if err := c.Cmd.Start(); err != nil {
		c.CloseLog()
		return err
	}

	c.status = StatusRunning

	return nil
}

// Success just proxies the function call to the command.ProcessState struct.
func (c *Command) Success() bool {
	return c.Cmd.ProcessState.Success()
}

// NewCommand returns an initalized command pointer.
func NewCommand(log io.WriteCloser, version, name string, args ...string) *Command {
	cmd := &Command{
		Log:     log,
		Cmd:     exec.Command(name, args...),
		Version: version,

		status:   StatusIdle,
		execName: name,
		execArgs: args,
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
	return nu.Version, nil
}
