package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"

	log "github.com/Sirupsen/logrus"
	"github.com/WiseGrowth/wisebot-operator/command"
	"github.com/WiseGrowth/wisebot-operator/git"
	"github.com/WiseGrowth/wisebot-operator/iot"
	homedir "github.com/mitchellh/go-homedir"
)

var (
	services ServiceStore
)

const (
	// wisebotServiceName = "wisebot-core"
	// wisebotCoreRepoPath    = "~/wisebot-core"
	// wisebotCoreRepoRemote  = "git@github.com:wisegrowth/wisebot-core.git"
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
)

func init() {
	var err error
	log.SetLevel(log.DebugLevel)

	wisebotConfigExpandedPath, err = homedir.Expand(wisebotConfigPath)
	check(err)

	wisebotCoreRepoExpandedPath, err = homedir.Expand(wisebotCoreRepoPath)
	check(err)

	// ----- Load wisebot config
	wisebotConfig, err = loadConfig(wisebotConfigExpandedPath)
	if err == errNoConfigFile {
		log.Errorf("The %q config file does not exists", wisebotConfigExpandedPath)
		os.Exit(1)
	}
	check(err)

	healthzPublishableTopic = fmt.Sprintf("/operator/%s/healthz", wisebotConfig.WisebotID)
}

func main() {
	// ----- Initialize git repos
	wisebotCoreRepo = git.NewRepo(
		wisebotCoreRepoExpandedPath,
		wisebotCoreRepoRemote,
		git.NpmInstallHook,
		git.NpmPruneHook,
	)

	const update = false
	check(wisebotCoreRepo.Bootstrap(update))

	// ----- Initialize commands
	wisebotCoreCommand := command.NewCommand(
		nil,
		wisebotCoreRepo.CurrentHead(),
		"node",
		wisebotCoreRepoExpandedPath+"/build/app/index.js",
	)
	// TODO: add ble command

	// Append services to global store
	services.Save(wisebotServiceName, wisebotCoreCommand, wisebotCoreRepo)

	// ----- Initialize MQTT connection
	cert, err := wisebotConfig.getTLSCertificate()
	check(err)

	client, err := iot.NewClient(
		iot.SetHost(iotHost),
		iot.SetCertificate(*cert),
	)
	check(err)

	// ----- Start application
	check(services.Start())
	check(client.Connect())

	// ----- Subscribe to MQTT topics
	check(client.Subscribe("/operator/"+wisebotConfig.WisebotID+"/healthz", healthzMQTTHandler))
	check(client.Subscribe("/operator/"+wisebotConfig.WisebotID+"/start", startCommandMQTTHandler))
	check(client.Subscribe("/operator/"+wisebotConfig.WisebotID+"/stop", stopCommandMQTTHandler))

	check(client.Subscribe("/operator/"+wisebotConfig.WisebotID+"/process-update", updateCommandMQTTHandler))

	// ----- Gracefully shutdown
	quit := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		client.Disconnect(250)
		log.Info("[MQTT] Disconnected")

		services.Stop()

		// rasp.TurnOffPins()
		quit <- struct{}{}
	}()
	<-quit
	log.Info("Done")
}

func check(err error) {
	if err != nil {
		debug.PrintStack()
		log.Fatal(err)
		os.Exit(1)
	}
}
