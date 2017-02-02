package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
)

const (
	weisebotLogPath = "wisebot.logs"
)

type command struct {
	Log io.WriteCloser
	Cmd *exec.Cmd
}

type commands []*command

func (c *commands) Add(cmd *command) {
	*c = append(*c, cmd)
}

func (c *commands) Start() error {
	for _, cmd := range *c {
		if err := cmd.Start(); err != nil {
			return err
		}
	}

	return nil
}

func (c *commands) Stop() error {
	for _, cmd := range *c {
		if err := cmd.Stop(); err != nil {
			return err
		}
	}

	return nil
}

func newCommand(log io.WriteCloser, name string, args ...string) *command {
	return &command{
		Log: log,
		Cmd: exec.Command(name, args...),
	}
}

func (c *command) Stop() error {
	defer c.Log.Close()
	return c.Cmd.Process.Kill()
}

func (c *command) Start() error {
	out, err := c.Cmd.StdoutPipe()
	if err != nil {
		c.Log.Close()
		return err
	}

	log.Println("Starting", c.Cmd.Args)

	go func() {
		for {
			r := bufio.NewReader(out)
			l, _, err := r.ReadLine()
			if err != nil {
				if err != io.EOF {
					c.Log.Close()
					panic(err)
				}
			}

			c.Log.Write(l)
			c.Log.Write([]byte("\n"))
		}
	}()

	if err := c.Cmd.Start(); err != nil {
		c.Log.Close()
		return err
	}

	return nil
}

var (
	cmds commands
)

func newFile(name string) (*os.File, error) {
	file, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			file, err = os.Create(name)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return file, err
}

func main() {
	logFile, err := newFile("wisebot.logs")
	if err != nil {
		panic(err)
	}

	wisebot1 := newCommand(logFile, "node", "wisebot1.js")
	wisebot2 := newCommand(logFile, "node", "wisebot2.js")

	cmds.Add(wisebot1)
	cmds.Add(wisebot2)

	if err := cmds.Start(); err != nil {
		panic(err)
	}

	quit := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			fmt.Println("Signal received", sig)
			if err := cmds.Stop(); err != nil {
				log.Println(err)
			}
			quit <- struct{}{}
		}
	}()

	<-quit
	fmt.Println("Quit")
}
