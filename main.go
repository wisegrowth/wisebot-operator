package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/WiseGrowth/wisebot-operator/command"
	"github.com/WiseGrowth/wisebot-operator/daemon"
	"github.com/WiseGrowth/wisebot-operator/git"
	"github.com/WiseGrowth/wisebot-operator/iot"
	"github.com/WiseGrowth/wisebot-operator/logger"
	"github.com/WiseGrowth/wisebot-operator/rasp"
	homedir "github.com/mitchellh/go-homedir"
)

var (
	sentryDSN string

	operatorVersion string
)

const (
	wisebotCoreServiceName = "wisebot-core"
	wisebotCoreRepoPath    = "~/wisebot-core"
	wisebotCoreRepoRemote  = "git@github.com:wisegrowth/test.git"

	bleServiceName = "wisebot-ble"
	bleRepoPath    = "~/wisebot-ble"
	bleRepoRemote  = "git@github.com:wisegrowth/wisebot-ble.git"

	ledDaemonName       = "led"
	ledDaemonRepoPath   = "~/wisebot-led-indicator"
	ledDaemonRepoRemote = "git@github.com:wisegrowth/wisebot-led-indicator.git"

	wisebotConfigPath = "~/.config/wisebot/config.json"
	wisebotLogPath    = "~/.wisebot/logs/operator.log"
)

var (
	wisebotCoreRepoExpandedPath string
	bleRepoExpandedPath         string
	ledDaemonRepoExpandedPath   string

	wisebotCoreRepo *git.Repo
	wisebotConfig   *config
	wisebotLogger   io.WriteCloser

	healthzPublishableTopic string

	httpServer     *http.Server
	processManager *ProcessManager
	daemonStore    *daemon.Store
)

func init() {
	var err error
	processManager = new(ProcessManager)
	daemonStore = new(daemon.Store)

	wisebotCoreRepoExpandedPath, err = homedir.Expand(wisebotCoreRepoPath)
	check(err)

	ledDaemonRepoExpandedPath, err = homedir.Expand(ledDaemonRepoPath)
	check(err)

	// ----- Load wisebot config
	wisebotConfig, err = loadConfig(wisebotConfigPath)
	check(err)

	healthzPublishableTopic = fmt.Sprintf("/operator/%s/healthz", wisebotConfig.WisebotID)

	wisebotLogger, err = newFile(wisebotLogPath)
	check(err)
}

func main() {
	defer wisebotLogger.Close()
	check(logger.Init(wisebotLogger, wisebotConfig.WisebotID, sentryDSN))

	log := logger.GetLogger().WithField("version", operatorVersion)
	log.Info("Starting")

	// ----- Initialize git repos
	ledDaemonRepo := git.NewRepo(
		ledDaemonRepoExpandedPath,
		ledDaemonRepoRemote,
		git.NpmInstallHook,
		git.NpmPruneHook,
	)

	wisebotCoreRepo = git.NewRepo(
		wisebotCoreRepoExpandedPath,
		wisebotCoreRepoRemote,
		git.NpmInstallHook,
		git.NpmPruneHook,
	)

	// ----- Initialize daemons
	if runtime.GOOS != "darwin" {
		d, err := daemon.NewDaemon(ledDaemonName, ledDaemonRepo)
		check(err)
		daemonStore.Save(d)
	}

	// ----- Initialize commands
	wisebotCoreCommand := command.NewCommand(
		"node",
		wisebotCoreRepoExpandedPath+"/build/app/index.js",
	)

	// ----- Initialize MQTT client
	cert, err := wisebotConfig.getTLSCertificate()
	check(err)

	mqttClient, err := iot.NewClient(
		iot.SetHost(wisebotConfig.AWSIOTHost),
		iot.SetCertificate(*cert),
		iot.SetClientID("op-"+wisebotConfig.WisebotID),
	)
	check(err)

	// We check internet connection before starting the web server, if there is a
	// critical error, there is no reason to start the web server or run
	// gracefullShutdown function since the operator should exit because an
	// unexpected fatal error.
	log.Debug("Checking wifi connection")
	isConnected, err := rasp.IsConnected()
	check(err)

	// ----- Append services to global store
	services := new(ServiceStore)
	services.Save(wisebotCoreServiceName, wisebotCoreCommand, wisebotCoreRepo)

	processManager = &ProcessManager{
		MQTTClient: mqttClient,
		Services:   services,
	}

	httpServer = NewHTTPServer()
	log.Info("Running server on: " + httpServer.Addr)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(err)
		}
	}()

	quit := make(chan struct{})
	log.Debug(fmt.Sprintf("Internet connection: %v", isConnected))
	const updateOnBootstrap = true
	if isConnected {
		check(processManager.KickOff(updateOnBootstrap))
		check(daemonStore.Bootstrap(updateOnBootstrap))
		// This should be removed, since wisebot-core will send this notification
	} else {
		tick := time.NewTicker(30 * time.Second)
		go func() {
			for range tick.C {
				isConnected, _ := rasp.IsConnected()
				if isConnected {
					if err := processManager.KickOff(updateOnBootstrap); err != nil {
						log.Error(err)
						quit <- struct{}{}
						return
					}
					if err := daemonStore.Bootstrap(updateOnBootstrap); err != nil {
						log.Error(err)
						quit <- struct{}{}
						return
					}

					return
				}
			}
		}()
	}
	listenInterrupt(quit)
	<-quit
	gracefullShutdown()
	wisebotLogger.Close()
	log.Info("Done")
}

func listenInterrupt(quit chan struct{}) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		quit <- struct{}{}
	}()
}

func gracefullShutdown() {
	log := logger.GetLogger()
	if err := httpServer.Shutdown(nil); err != nil {
		log.Error(err.Error())
	}
	processManager.Stop()
}

func check(err error) {
	if err != nil {
		log := logger.GetLogger()

		switch (err).(type) {
		case *exec.ExitError:
			e, _ := (err).(*exec.ExitError)
			stderr := bytes.TrimSpace(e.Stderr)
			log.WithField("stderr", string(stderr)).Fatal(err)
		default:
			log.Fatal(err)
		}
		wisebotLogger.Close()
	}
}
