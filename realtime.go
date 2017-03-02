package main

import (
	"encoding/json"

	"github.com/WiseGrowth/wisebot-operator/logger"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

func healthzMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()

	log := logger.GetLogger().WithField("topic", topic)
	log.Info("Message received")

	responseBytes, _ := json.Marshal(newHealthResponse())

	token := client.Publish(topic+":response", byte(1), false, responseBytes)
	if token.Wait() && token.Error() != nil {
		log.Error(token.Error())
	}
}

func updateCommandMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer func() {
		responseBytes, _ := json.Marshal(newHealthResponse())

		token := client.Publish(healthzPublishableTopic+":response", byte(1), false, responseBytes)
		if token.Wait() && token.Error() != nil {
			log.Error(token.Error())
		}
	}()

	log.Info("Message received")

	payload := struct {
		Process struct {
			Name string `json:"name"`
		} `json:"process"`
	}{}

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := services.Update(payload.Process.Name); err != nil {
		log.Error(err)
		return
	}
}

func stopCommandMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer func() {
		responseBytes, _ := json.Marshal(newHealthResponse())

		token := client.Publish(healthzPublishableTopic+":response", byte(1), false, responseBytes)
		if token.Wait() && token.Error() != nil {
			log.Error(token.Error())
		}
	}()

	log.Info("Message received")

	payload := struct {
		Process struct {
			Name string `json:"name"`
		} `json:"process"`
	}{}

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := services.StopService(payload.Process.Name); err != nil {
		log.Error(err)
		return
	}
}

func startCommandMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer func() {
		responseBytes, _ := json.Marshal(newHealthResponse())

		token := client.Publish(healthzPublishableTopic+":response", byte(1), false, responseBytes)
		if token.Wait() && token.Error() != nil {
			log.Error(token.Error())
		}
	}()
	log.Info("Message received")

	payload := struct {
		Process struct {
			Name string `json:"name"`
		} `json:"process"`
	}{}

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := services.StartService(payload.Process.Name); err != nil {
		log.Error(err)
		return
	}
}
