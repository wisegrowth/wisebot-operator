package main

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type healthzResponse struct {
	Data ServiceStore `json:"data"`
}

func healthzMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()

	logger := log.WithField("topic", topic)
	logger.Info("Message received")

	res := &healthzResponse{Data: services}
	responseBytes, _ := json.Marshal(res)

	token := client.Publish(topic+":response", byte(1), false, responseBytes)
	if token.Wait() && token.Error() != nil {
		log.Error(token.Error())
	}
}

func updateCommandMQTTHandler(client MQTT.Client, message MQTT.Message) {
	defer func() {
		res := &healthzResponse{Data: services}
		responseBytes, _ := json.Marshal(res)

		token := client.Publish(healthzPublishableTopic+":response", byte(1), false, responseBytes)
		if token.Wait() && token.Error() != nil {
			log.Error(token.Error())
		}
	}()

	topic := message.Topic()

	logger := log.WithField("topic", topic)
	logger.Info("Message received")

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
	defer func() {
		res := &healthzResponse{Data: services}
		responseBytes, _ := json.Marshal(res)

		token := client.Publish(healthzPublishableTopic+":response", byte(1), false, responseBytes)
		if token.Wait() && token.Error() != nil {
			log.Error(token.Error())
		}
	}()

	topic := message.Topic()

	logger := log.WithField("topic", topic)
	logger.Info("Message received")

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
	defer func() {
		res := &healthzResponse{Data: services}
		responseBytes, _ := json.Marshal(res)

		token := client.Publish(healthzPublishableTopic+":response", byte(1), false, responseBytes)
		if token.Wait() && token.Error() != nil {
			log.Error(token.Error())
		}
	}()

	topic := message.Topic()

	logger := log.WithField("topic", topic)
	logger.Info("Message received")

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
