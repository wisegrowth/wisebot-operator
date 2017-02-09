package main

import (
	"encoding/json"

	"github.com/WiseGrowth/operator/command"

	log "github.com/Sirupsen/logrus"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

func healthzMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()

	logger := log.WithField("topic", topic)
	logger.Info("Message received")

	bytes, _ := json.Marshal(&struct {
		Data command.Commands `json:"data"`
	}{commands})

	token := client.Publish(topic+":response", byte(1), false, bytes)
	if token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
}

func updateCommandMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()

	logger := log.WithField("topic", topic)
	logger.Info("Message received")

	// TODO: define payload
	payload := struct {
		Data struct {
			Process string `json:"process"`
		} `json:"data"`
	}{}

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := commands.Update(payload.Data.Process); err != nil {
		log.Error(err)
		return
	}
}

func stopCommandMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()

	logger := log.WithField("topic", topic)
	logger.Info("Message received")

	// TODO: define payload
	payload := struct {
		Data struct {
			Process string `json:"process"`
		} `json:"data"`
	}{}

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := commands.StopCommand(payload.Data.Process); err != nil {
		log.Error(err)
		return
	}
}

func startCommandMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()

	logger := log.WithField("topic", topic)
	logger.Info("Message received")

	// TODO: define payload
	payload := struct {
		Data struct {
			Process string `json:"process"`
		} `json:"data"`
	}{}

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := commands.StartCommand(payload.Data.Process); err != nil {
		log.Error(err)
		return
	}
}
