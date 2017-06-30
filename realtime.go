package main

import (
	"encoding/json"

	"github.com/WiseGrowth/go-wisebot/logger"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

// actionPayload represents the received payload for starting, stoping and
// restarting daemons and services.
type actionPayload struct {
	Name string `json:"name"`
}

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

func startServiceMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)
	log.Info("Message received")

	payload := new(actionPayload)

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := processManager.Services.StartService(payload.Name); err != nil {
		log.Error(err)
		return
	}
}

func startDaemonMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)
	log.Info("Message received")

	payload := new(actionPayload)

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := daemonStore.StartDaemon(payload.Name); err != nil {
		log.Error(err)
		return
	}
}

func stopServiceMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)

	log.Info("Message received")

	payload := new(actionPayload)

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := processManager.Services.StopService(payload.Name); err != nil {
		log.Error(err)
		return
	}
}

func stopDaemonMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)

	log.Info("Message received")

	payload := new(actionPayload)

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := daemonStore.StopDaemon(payload.Name); err != nil {
		log.Error(err)
		return
	}
}

func updateServiceMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)

	log.Info("Message received")

	payload := new(actionPayload)

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := processManager.Services.Update(payload.Name); err != nil {
		log.Error(err)
		return
	}
}

func updateDaemonMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)

	log.Info("Message received")

	payload := new(actionPayload)

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := daemonStore.Update(payload.Name); err != nil {
		log.Error(err)
		return
	}
}

func restartDaemonMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)
	log.Info("Message received")

	payload := new(actionPayload)

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := daemonStore.RestartDaemon(payload.Name); err != nil {
		log.Error(err)
		return
	}
}

func publishHealthz(client MQTT.Client, log *logrus.Entry) {
	responseBytes, _ := json.Marshal(newHealthResponse())

	token := client.Publish(healthzPublishableTopic+":response", byte(1), false, responseBytes)
	if token.Wait() && token.Error() != nil {
		log.Error(token.Error())
	}
}
