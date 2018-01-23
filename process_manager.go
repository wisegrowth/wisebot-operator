package main

import (
	"sync"

	"github.com/WiseGrowth/go-wisebot/logger"
	"github.com/WiseGrowth/wisebot-operator/iot"
)

// ProcessManager is in charge of starting and stoping the processes.
type ProcessManager struct {
	sync.Mutex
	Services              *ServiceStore
	MQTTClient            *iot.Client
	servicesStarted       bool
	mqttConnectionStarted bool
}

// KickOffServices is a function that kicks off core subprocesses
// managed by the operator. `hasInternetConnection` param indicates wether the
// subprocesses and daemons source code can be updated by git or not.
// The subprocesses knows how to reconnect when they lose internet connection,
// so is not necessary to start them while being connected to the internet.
// The wrong value of `hasInternetConnection` can raise errors, so is mandatory
// to check if the device is online before executing this method.
func (pm *ProcessManager) KickOffServices(hasInternetConnection bool) error {
	pm.Lock()
	defer pm.Unlock()

	log := logger.GetLogger()
	log.Debug("Bootstraping and starting services")

	if pm.servicesStarted {
		log.Debug("Process already started, ignoring KickOffServices()")
		return nil
	}

	if err := pm.bootstrapServices(hasInternetConnection); err != nil {
		return err
	}

	pm.servicesStarted = true

	log.Debug("Bootstraping done")
	return nil
}

// KickOffMQTTClient kicks off the mqtt operator connection. This function must
// to be called only if the operator has an internet connection, ergo, the
// device is online.
func (pm *ProcessManager) KickOffMQTTClient() error {
	pm.Lock()
	defer pm.Unlock()

	log := logger.GetLogger()
	log.Debug("Establishing mqtt connection")

	if pm.mqttConnectionStarted {
		log.Debug("Mqtt connection already started, ignoring KickOffMQTTClient()")
		return nil
	}

	if err := pm.bootstrapMQTTClient(); err != nil {
		return err
	}

	pm.mqttConnectionStarted = true

	log.Debug("Mqtt connection established")
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
	if err := pm.MQTTClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/service-start", startServiceMQTTHandler); err != nil {
		return err
	}
	if err := pm.MQTTClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/service-stop", stopServiceMQTTHandler); err != nil {
		return err
	}
	if err := pm.MQTTClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/service-update", updateServiceMQTTHandler); err != nil {
		return err
	}
	if err := pm.MQTTClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/service-restart", restartServiceMQTTHandler); err != nil {
		return err
	}
	if err := pm.MQTTClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/daemon-start", startDaemonMQTTHandler); err != nil {
		return err
	}
	if err := pm.MQTTClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/daemon-stop", stopDaemonMQTTHandler); err != nil {
		return err
	}
	if err := pm.MQTTClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/daemon-update", updateDaemonMQTTHandler); err != nil {
		return err
	}
	if err := pm.MQTTClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/daemon-restart", restartDaemonMQTTHandler); err != nil {
		return err
	}
	if err := pm.MQTTClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/update", updateOperatorMQTTHandler); err != nil {
		return err
	}

	return nil
}
