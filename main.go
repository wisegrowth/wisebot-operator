package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/WiseGrowth/operator/command"
)

const (
	weisebotLogPath = "wisebot.logs"
)

var (
	cmds command.Commands
)

func main() {
	logFile, err := newFile("wisebot.logs")
	if err != nil {
		panic(err)
	}

	wisebot1 := command.NewCommand(logFile, "node", "wisebot1.js")
	wisebot2 := command.NewCommand(logFile, "node", "wisebot2.js")

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
