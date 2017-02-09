package main

import (
	"encoding/json"
	"os"
	"os/signal"

	log "github.com/Sirupsen/logrus"
	"github.com/WiseGrowth/operator/command"
	"github.com/WiseGrowth/operator/git"
	"github.com/WiseGrowth/operator/iot"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var (
	commands command.Commands
)

func init() {
	log.SetLevel(log.DebugLevel)

	commands = make(command.Commands)
	commands.Add(
		command.NewCommand(nil, "test-cmd", "echo", "hello world"),
	)
}

func healthz(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()

	logger := log.WithField("topic", topic)
	logger.Info("Received message", message.Payload())

	bytes, _ := json.Marshal(&struct {
		Data command.Commands `json:"data"`
	}{commands})

	token := client.Publish(topic+":response", byte(1), false, bytes)
	if token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
}

func main() {
	r := git.NewRepo(
		"wisebots-api",
		"git@github.com:wisegrowth/wisebots-api.git",
	)

	r.AddPostReceiveHooks(
		r.NpmInstall(),
		r.NpmPrune(),
	)

	wisebotConfig, err := loadConfig("./config.json") // TODO: use real path
	check(err)
	cert, err := wisebotConfig.getTLSCertificate()
	check(err)

	client, err := iot.NewClient(
		iot.SetHost("a55lp0huv9vtb.iot.us-west-2.amazonaws.com"),
		iot.SetCertificate(*cert),
	)
	check(err)
	check(client.Connect())

	check(client.Subscribe("/operator/"+wisebotConfig.WisebotID+"/healthz", healthz))

	check(commands.Start())

	check(r.Bootstrap())

	quit := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		client.Disconnect(250)
		log.Info("[MQTT] Disconnected")

		commands.Stop()

		quit <- struct{}{}
	}()
	<-quit
	log.Debug("Done")
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
