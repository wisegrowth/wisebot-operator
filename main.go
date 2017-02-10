package main

import (
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
	wisebotCoreCommandSlug = "wisebots-api"
	wisebotCoreRepoPath    = "~/wisebots-api"
	wisebotCoreRepoRemote  = "git@github.com:wisegrowth/wisebots-api.git"

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
)

func init() {
	var err error
	log.SetLevel(log.DebugLevel)

	wisebotConfigExpandedPath, err = homedir.Expand(wisebotConfigPath)
	check(err)

	wisebotCoreRepoExpandedPath, err = homedir.Expand(wisebotCoreRepoPath)
	check(err)

	// TODO: add ble command
}

func main() {
	wisebotCoreRepo = git.NewRepo(
		wisebotCoreRepoExpandedPath,
		wisebotCoreRepoRemote,
	)

	wisebotCoreRepo.AddPostReceiveHooks(
		wisebotCoreRepo.NpmInstall(),
		wisebotCoreRepo.NpmPrune(),
	)

	check(wisebotCoreRepo.Bootstrap())

	wisebotCoreCommand := command.NewCommand(nil, wisebotCoreCommandSlug, wisebotCoreRepo.CurrentHead(), "node", wisebotCoreRepoExpandedPath+"/build/app/index.js")
	wisebotCoreCommand.Updater = wisebotCoreRepo

	commands = make(command.Commands)
	commands.Add(wisebotCoreCommand)

	wisebotConfig, err := loadConfig(wisebotConfigExpandedPath)
	check(err)
	cert, err := wisebotConfig.getTLSCertificate()
	check(err)

	client, err := iot.NewClient(
		iot.SetHost(iotHost),
		iot.SetCertificate(*cert),
	)

	check(err)

	check(commands.Start())
	check(client.Connect())

	check(client.Subscribe("/operator/"+wisebotConfig.WisebotID+"/healthz", healthzMQTTHandler))
	check(client.Subscribe("/operator/"+wisebotConfig.WisebotID+"/start", startCommandMQTTHandler))
	check(client.Subscribe("/operator/"+wisebotConfig.WisebotID+"/stop", stopCommandMQTTHandler))

	check(client.Subscribe("/operator/"+wisebotConfig.WisebotID+"/process-update", updateCommandMQTTHandler))

	quit := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		client.Disconnect(250)
		log.Info("[MQTT] Disconnected")

		commands.Stop()

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
