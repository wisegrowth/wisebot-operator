package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"github.com/WiseGrowth/operator/command"
	"github.com/WiseGrowth/operator/iot"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var (
	wisebotID string

	commands *command.Commands
)

func init() {
	wisebotID = "wisebot-id"
	iot.SetDebug(true)
}

func onMessageReceived(client MQTT.Client, message MQTT.Message) {
	fmt.Printf("Received message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())
}

func healthz(client MQTT.Client, message MQTT.Message) {
	fmt.Printf("Received message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())

	bytes, _ := json.Marshal(&struct {
		Data *command.Commands `json:"data"`
	}{commands})

	token := client.Publish("/operator/"+wisebotID+"/healthz:response", byte(1), false, bytes)
	if token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
}

func init() {
	commands = new(command.Commands)
	commands.Add(
		command.NewCommand(nil, "wise-devide-manager", "echo", "helloworld"),
	)
	// commands.Add(
	// 	command.NewCommand(nil, "sleeping", "sleep", "20"),
	// )
}

func main() {
	client, err := iot.NewClient(
		iot.SetHost("a55lp0huv9vtb.iot.us-west-2.amazonaws.com"),
		iot.SetCert("tls/chechitodelboom.cert.pem"),
		iot.SetKey("tls/chechitodelboom.private.key"),
		iot.SetOnConnect(func(c *iot.Client) {
			c.Subscribe("/operator/"+wisebotID+"/healthz", healthz)
		}),
	)
	if err != nil {
		panic(err)
	}

	check(client.Connect())

	check(commands.Start())

	quit := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		client.Disconnect(250)
		fmt.Println("[MQTT] Disconnected")

		// commands.Stop()

		quit <- struct{}{}
	}()
	<-quit
	fmt.Println("Done")
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
