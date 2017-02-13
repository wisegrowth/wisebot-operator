package main

import (
	"encoding/json"

	"github.com/WiseGrowth/wisebot-operator/command"
	"github.com/WiseGrowth/wisebot-operator/git"

	log "github.com/Sirupsen/logrus"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

func healthzMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()

	logger := log.WithField("topic", topic)
	logger.Info("Message received")

	responseBytes, _ := json.Marshal(&struct {
		Data command.Commands `json:"data"`
	}{commands})

	token := client.Publish(topic+":response", byte(1), false, responseBytes)
	if token.Wait() && token.Error() != nil {
		log.Error(token.Error())
	}
}

type updateCommandResponse struct {
	Data command.Commands `json:"data"`
	Meta struct {
		Repos []*git.Repo `json:"repos"`
	} `json:"meta"`
}

func updateCommandMQTTHandler(client MQTT.Client, message MQTT.Message) {
	defer func() {
		res := &updateCommandResponse{
			Data: commands,
		}
		res.Meta.Repos = []*git.Repo{wisebotCoreRepo}
		responseBytes, _ := json.Marshal(res)

		token := client.Publish(healthzPublishableTopic+":response", byte(1), false, responseBytes)
		if token.Wait() && token.Error() != nil {
			log.Error(token.Error())
		}
	}()

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
	defer func() {
		res := &updateCommandResponse{
			Data: commands,
		}
		res.Meta.Repos = []*git.Repo{wisebotCoreRepo}
		responseBytes, _ := json.Marshal(res)

		token := client.Publish(healthzPublishableTopic+":response", byte(1), false, responseBytes)
		if token.Wait() && token.Error() != nil {
			log.Error(token.Error())
		}
	}()

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
	defer func() {
		res := &updateCommandResponse{
			Data: commands,
		}
		res.Meta.Repos = []*git.Repo{wisebotCoreRepo}
		responseBytes, _ := json.Marshal(res)

		token := client.Publish(healthzPublishableTopic+":response", byte(1), false, responseBytes)
		if token.Wait() && token.Error() != nil {
			log.Error(token.Error())
		}
	}()

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
