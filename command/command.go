package command

import (
	"bufio"
	"io"
	"os"
	"os/exec"
)

// Command represents a os level command, which can also
// receive a logger file in order to dump the output to it.
type Command struct {
	Log io.WriteCloser
	Cmd *exec.Cmd
}

// CloseLogger safely close the command's logger.
// If the logger is just os.Stdout, it does not
// close it.
func (c *Command) CloseLogger() error {
	if c.Log == nil || c.Log == os.Stdout {
		return nil
	}

	return c.Log.Close()
}

// Stop stops the command and closes the log file
// if exists.
func (c *Command) Stop() error {
	if c.Log != nil {
		defer c.CloseLogger()
	}
	return c.Cmd.Process.Kill()
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
	out, err := c.Cmd.StdoutPipe()
	if err != nil {
		c.CloseLogger()
		return err
	}

	go func() {
		if c.Log == nil {
			return
		}

		for {
			r := bufio.NewReader(out)
			l, _, err := r.ReadLine()
			if err != nil {
				if err != io.EOF {
					c.CloseLogger()
					panic(err)
				}
			}

			c.Log.Write(l)
			c.Log.Write([]byte("\n"))
		}
	}()

	if err := c.Cmd.Start(); err != nil {
		c.CloseLogger()
		return err
	}

	return nil
}

// Success just proxies the function call to the
// command.ProcessState struct.
func (c *Command) Success() bool {
	return c.Cmd.ProcessState.Success()
}

// Commands represents a set of commands.
// It has convinient methods to run and stop all
// commands.
type Commands []*Command

// Add adds the received command into the commands list
func (c *Commands) Add(cmd *Command) {
	*c = append(*c, cmd)
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
func NewCommand(log io.WriteCloser, name string, args ...string) *Command {
	return &Command{
		Log: log,
		Cmd: exec.Command(name, args...),
	}
}
