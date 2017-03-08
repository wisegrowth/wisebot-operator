package main

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"
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

func startServiceMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)
	log.Info("Message received")

	payload := struct {
		Service struct {
			Name string `json:"name"`
		} `json:"service"`
	}{}

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := processManager.Services.StartService(payload.Service.Name); err != nil {
		log.Error(err)
		return
	}
}

func startDaemonMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)
	log.Info("Message received")

	payload := struct {
		Daemon struct {
			Name string `json:"name"`
		} `json:"daemon"`
	}{}

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := daemonStore.StartDaemon(payload.Daemon.Name); err != nil {
		log.Error(err)
		return
	}
}

func stopServiceMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)

	log.Info("Message received")

	payload := struct {
		Service struct {
			Name string `json:"name"`
		} `json:"service"`
	}{}

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := processManager.Services.StopService(payload.Service.Name); err != nil {
		log.Error(err)
		return
	}
}

func stopDaemonMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)

	log.Info("Message received")

	payload := struct {
		Daemon struct {
			Name string `json:"name"`
		} `json:"daemon"`
	}{}

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := daemonStore.StopDaemon(payload.Daemon.Name); err != nil {
		log.Error(err)
		return
	}
}

func updateServiceMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)

	log.Info("Message received")

	payload := struct {
		Service struct {
			Name string `json:"name"`
		} `json:"service"`
	}{}

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := processManager.Services.Update(payload.Service.Name); err != nil {
		log.Error(err)
		return
	}
}

func updateDaemonMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)

	log.Info("Message received")

	payload := struct {
		Daemon struct {
			Name string `json:"name"`
		} `json:"daemon"`
	}{}

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := daemonStore.Update(payload.Daemon.Name); err != nil {
		log.Error(err)
		return
	}
}

func restartDaemonMQTTHandler(client MQTT.Client, message MQTT.Message) {
	topic := message.Topic()
	log := logger.GetLogger().WithField("topic", topic)

	defer publishHealthz(client, log)
	log.Info("Message received")

	payload := struct {
		Daemon struct {
			Name string `json:"name"`
		} `json:"daemon"`
	}{}

	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Error(err)
		return
	}

	if err := daemonStore.RestartDaemon(payload.Daemon.Name); err != nil {
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
