package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/WiseGrowth/wisebot-operator/command"
	"github.com/WiseGrowth/wisebot-operator/git"
	"github.com/WiseGrowth/wisebot-operator/iot"
	"github.com/WiseGrowth/wisebot-operator/logger"
	"github.com/WiseGrowth/wisebot-operator/rasp"
	homedir "github.com/mitchellh/go-homedir"
)

var (
	services *ServiceStore

	sentryDSN string
)

const (
	wisebotServiceName    = "wisebot-test"
	wisebotCoreRepoPath   = "~/wisebot-test"
	wisebotCoreRepoRemote = "git@github.com:wisegrowth/test.git"

	bleServiceName = "wisebot-ble"
	bleRepoPath    = "~/wisebot-ble"
	bleRepoRemote  = "git@github.com:wisegrowth/wisebot-ble.git"

	wisebotConfigPath = "~/.config/wisebot-operator/config.json"

	iotHost = "a55lp0huv9vtb.iot.us-west-2.amazonaws.com"
)

var (
	wisebotConfigExpandedPath   string
	wisebotCoreRepoExpandedPath string
	bleRepoExpandedPath         string

	wisebotCoreRepo *git.Repo
	wisebotConfig   *config

	healthzPublishableTopic string

	mqttClient *iot.Client
	httpServer *http.Server
)

func init() {
	var err error
	services = new(ServiceStore)

	wisebotConfigExpandedPath, err = homedir.Expand(wisebotConfigPath)
	check(err)

	wisebotCoreRepoExpandedPath, err = homedir.Expand(wisebotCoreRepoPath)
	check(err)

	// ----- Load wisebot config
	wisebotConfig, err = loadConfig(wisebotConfigExpandedPath)
	check(err)

	healthzPublishableTopic = fmt.Sprintf("/operator/%s/healthz", wisebotConfig.WisebotID)

	check(logger.Init(wisebotConfig.WisebotID, sentryDSN))

	// ----- Initialize MQTT client
	cert, err := wisebotConfig.getTLSCertificate()
	check(err)

	mqttClient, err = iot.NewClient(
		iot.SetHost(iotHost),
		iot.SetCertificate(*cert),
	)
	check(err)
}

func main() {
	log := logger.GetLogger()

	// ----- Initialize git repos
	wisebotCoreRepo = git.NewRepo(
		wisebotCoreRepoExpandedPath,
		wisebotCoreRepoRemote,
		git.NpmInstallHook,
		git.NpmPruneHook,
	)

	// ----- Initialize commands
	wisebotCoreCommand := command.NewCommand(
		nil,
		wisebotCoreRepo.CurrentHead(),
		"node",
		wisebotCoreRepoExpandedPath+"/build/app/index.js",
	)

	// ----- Append services to global store
	services.Save(wisebotServiceName, wisebotCoreCommand, wisebotCoreRepo)

	httpServer = NewHTTPServer()
	log.Info("Running server on: " + httpServer.Addr)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(err)
		}
	}()

	log.Debug("Checking wifi connection")
	isConnected, err := rasp.IsConnected()
	check(err)

	httpClient := &http.Client{}

	log.Debug(fmt.Sprintf("Internet connection: %v", isConnected))
	if isConnected {
		httpClient.Do(buildRequest("green"))
		log.Debug("Bootstraping and starting services")
		const update = true
		check(bootstrapServices(update))
		check(bootstrapMQTTClient())
		log.Debug("Bootstraping done")
	} else {
		httpClient.Do(buildRequest("blue"))
	}

	// ----- Gracefully shutdown
	quit := make(chan struct{})
	listenInterrupt(quit)
	<-quit
	log.Info("Done")
}

func buildRequest(color string) *http.Request {
	payload := []byte(fmt.Sprintf("{\"color\": \"%s\"}", color))

	body := bytes.NewBuffer(payload)
	ledRequest, err := http.NewRequest("POST", "http://localhost:5001/set-color", body)
	check(err)

	ledRequest.Header.Set("Content-Type", "application/json")

	return ledRequest
}

func bootstrapServices(update bool) error {
	log := logger.GetLogger()

	log.Debug("Bootstraping repos")
	if err := services.Bootstrap(update); err != nil {
		return err
	}

	log.Debug("Starting commands")
	if err := services.Start(); err != nil {
		services.Stop()
		return err
	}

	return nil
}

func listenInterrupt(quit chan struct{}) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c

		log := logger.GetLogger()
		if err := httpServer.Shutdown(nil); err != nil {
			log.Error(err.Error())
		}

		mqttClient.Disconnect(250)
		log.Info("[MQTT] Disconnected")

		services.Stop()

		quit <- struct{}{}
	}()
}

func bootstrapMQTTClient() error {
	log := logger.GetLogger()

	log.Debug("Connecting to MQTT Broker")
	if err := mqttClient.Connect(); err != nil {
		return err
	}

	log.Debug("Subscribing topics")
	// ----- Subscribe to MQTT topics
	if err := mqttClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/healthz", healthzMQTTHandler); err != nil {
		return err
	}
	if err := mqttClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/start", startCommandMQTTHandler); err != nil {
		return err
	}
	if err := mqttClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/stop", stopCommandMQTTHandler); err != nil {
		return err
	}
	if err := mqttClient.Subscribe("/operator/"+wisebotConfig.WisebotID+"/process-update", updateCommandMQTTHandler); err != nil {
		return err
	}

	return nil
}

func check(err error) {
	if err != nil {
		debug.PrintStack()
		logger.GetLogger().Fatal(err)
	}
}
