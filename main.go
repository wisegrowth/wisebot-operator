package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"

	"github.com/WiseGrowth/wisebot-operator/command"
	"github.com/WiseGrowth/wisebot-operator/git"
	"github.com/WiseGrowth/wisebot-operator/iot"
	"github.com/WiseGrowth/wisebot-operator/led"
	"github.com/WiseGrowth/wisebot-operator/logger"
	"github.com/WiseGrowth/wisebot-operator/rasp"
	homedir "github.com/mitchellh/go-homedir"
)

var (
	services *ServiceStore

	sentryDSN string
)

const (
	wisebotCoreServiceName = "wisebot-core"
	wisebotCoreRepoPath    = "~/wisebot-core"
	wisebotCoreRepoRemote  = "git@github.com:wisegrowth/wisebot-core.git"

	bleServiceName = "wisebot-ble"
	bleRepoPath    = "~/wisebot-ble"
	bleRepoRemote  = "git@github.com:wisegrowth/wisebot-ble.git"

	wisebotConfigPath = "~/.config/wisebot/config.json"
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

	wisebotLogger, err := newFile("wisebot.log")
	check(err)
	check(logger.Init(wisebotLogger, wisebotConfig.WisebotID, sentryDSN))

	// ----- Initialize MQTT client
	cert, err := wisebotConfig.getTLSCertificate()
	check(err)

	mqttClient, err = iot.NewClient(
		iot.SetHost(wisebotConfig.AWSIOTHost),
		iot.SetCertificate(*cert),
		iot.SetClientID("op-"+wisebotConfig.WisebotID),
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
		wisebotCoreRepo.CurrentHead(),
		"node",
		wisebotCoreRepoExpandedPath+"/build/app/index.js",
	)

	// ----- Append services to global store
	services.Save(wisebotCoreServiceName, wisebotCoreCommand, wisebotCoreRepo)

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

	log.Debug(fmt.Sprintf("Internet connection: %v", isConnected))
	if isConnected {
		if err := led.PostNetworkStatus(led.NetworkConnected); err != nil {
			log.Error(err)
		}

		log.Debug("Bootstraping and starting services")
		const update = true
		check(bootstrapServices(update))
		check(bootstrapMQTTClient())
		log.Debug("Bootstraping done")
	}

	// ----- Gracefully shutdown
	quit := make(chan struct{})
	listenInterrupt(quit)
	<-quit
	log.Info("Done")
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
		log := logger.GetLogger()

		switch (err).(type) {
		case *exec.ExitError:
			e, _ := (err).(*exec.ExitError)
			log.WithField("stderr", string(e.Stderr)).Fatal(err)
		default:
			log.Fatal(err)
		}
	}
}
