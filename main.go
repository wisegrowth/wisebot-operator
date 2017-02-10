package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"

	log "github.com/Sirupsen/logrus"
	"github.com/WiseGrowth/operator/command"
	"github.com/WiseGrowth/operator/git"
	"github.com/WiseGrowth/operator/iot"
	homedir "github.com/mitchellh/go-homedir"
)

var (
	commands command.Commands
)

const (
	// wisebotCoreCommandSlug = "wisebot-core"
	// wisebotCoreRepoPath    = "~/wisebot-core"
	// wisebotCoreRepoRemote  = "git@github.com:wisegrowth/wisebot-core.git"
	wisebotCoreCommandSlug = "wisebot-test"
	wisebotCoreRepoPath    = "~/Code/wg/test"
	wisebotCoreRepoRemote  = "git@github.com:wisegrowth/test.git"

	bleCommandSlug = "wisebot-ble"
	bleRepoPath    = "~/wisebot-ble"
	bleRepoRemote  = "git@github.com:wisegrowth/wisebot-ble.git"

	wisebotConfigPath = "./config.json" // TODO: use real path

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
	check(err)

	healthzPublishableTopic = fmt.Sprintf("/operator/%s/healthz", wisebotConfig.WisebotID)
}

func main() {
	// ----- Initialize git repos
	wisebotCoreRepo = git.NewRepo(
		wisebotCoreRepoExpandedPath,
		wisebotCoreRepoRemote,
	)

	wisebotCoreRepo.AddPostReceiveHooks(
		wisebotCoreRepo.NpmInstall(),
		wisebotCoreRepo.NpmPrune(),
	)

	const update = true
	check(wisebotCoreRepo.Bootstrap(update))

	// ----- Initialize commands
	wisebotCoreCommand := command.NewCommand(
		nil,
		wisebotCoreCommandSlug,
		wisebotCoreRepo.CurrentHead(),
		"node",
		wisebotCoreRepoExpandedPath+"/build/app/index.js",
	)
	wisebotCoreCommand.Updater = wisebotCoreRepo
	// TODO: add ble command

	// Append commands to the global variable
	commands = make(command.Commands)
	commands.Add(wisebotCoreCommand)

	// ----- Initialize MQTT connection
	cert, err := wisebotConfig.getTLSCertificate()
	check(err)

	client, err := iot.NewClient(
		iot.SetHost(iotHost),
		iot.SetCertificate(*cert),
	)
	check(err)

	// ----- Start application
	check(commands.Start())
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

		commands.Stop()

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
