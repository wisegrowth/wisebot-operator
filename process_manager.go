package main

import (
	"sync"

	"github.com/WiseGrowth/wisebot-operator/iot"
	"github.com/WiseGrowth/wisebot-operator/logger"
)

// ProcessManager is in charge of starting and stoping the processes.
type ProcessManager struct {
	sync.Mutex
	Services   *ServiceStore
	MQTTClient *iot.Client
	started    bool
}

// KickOff is a function that kicks off core subprocesses and messaging clients
// managed by the operator. This function must be called after checking that
// there is internet connection, otherwise, messaging client and subprocesses
// are going to fail.
// Both, subprocesses and messaging client knows how to reconnect when they
// lose connection, but they must be started while being online.
func (pm *ProcessManager) KickOff() error {
	pm.Lock()
	defer pm.Unlock()

	log := logger.GetLogger()
	log.Debug("Bootstraping and starting services")

	if pm.started {
		log.Debug("Process already started, ignoring start()")
		return nil
	}

	const update = true
	if err := pm.bootstrapServices(update); err != nil {
		return err
	}
	if err := pm.bootstrapMQTTClient(); err != nil {
		return err
	}

	pm.started = true

	log.Debug("Bootstraping done")
	return nil
}

// Stop stops `pm.ServiceStore` services and disconnects the MQTT Client.
func (pm *ProcessManager) Stop() {
	pm.Lock()
	defer pm.Unlock()

	log := logger.GetLogger()
	pm.MQTTClient.Disconnect(250)
	log.Info("[MQTT] Disconnected")
	pm.Services.Stop()
}

func (pm *ProcessManager) bootstrapServices(update bool) error {
	log := logger.GetLogger()

	log.Debug("Bootstraping repos")
	if err := pm.Services.Bootstrap(update); err != nil {
		return err
	}

	log.Debug("Starting commands")
	if err := pm.Services.Start(); err != nil {
		pm.Services.Stop()
		return err
	}

	return nil
}

func (pm *ProcessManager) bootstrapMQTTClient() error {
	log := logger.GetLogger()

	log.Debug("Connecting to MQTT Broker")
	if err := pm.MQTTClient.Connect(); err != nil {
		return err
	}

	log.Debug("Subscribing topics")
	// ----- Subscribe to MQTT topics
	if err := pm.MQTTClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/healthz", healthzMQTTHandler); err != nil {
		return err
	}
	if err := pm.MQTTClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/start", startCommandMQTTHandler); err != nil {
		return err
	}
	if err := pm.MQTTClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/stop", stopCommandMQTTHandler); err != nil {
		return err
	}
	if err := pm.MQTTClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/process-update", updateCommandMQTTHandler); err != nil {
		return err
	}

	return nil
}
