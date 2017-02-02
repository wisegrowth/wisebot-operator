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

func main() {
	wisebotLog, err := os.OpenFile(weisebotLogPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			wisebotLog, err = os.Create(weisebotLogPath)
			if err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	}
	defer wisebotLog.Close()

	wisebot := exec.Command("node", "wisebot.js")
	out, err := wisebot.StdoutPipe()
	if err != nil {
		panic(err)
	}

	log.Println("starting wisebot")

	go func() {
		for {
			r := bufio.NewReader(out)
			l, _, err := r.ReadLine()
			if err != nil {
				if err != io.EOF {
					panic(err)
				}
			}

			wisebotLog.Write(l)
			wisebotLog.WriteString("\n")
		}
	}()

	if err := wisebot.Start(); err != nil {
		panic(err)
	}

	quit := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			fmt.Println("Signal received", sig)
			wisebot.Process.Kill()
			quit <- struct{}{}
		}
	}()

	<-quit
	fmt.Println("Quit")
}
