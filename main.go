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

	"github.com/WiseGrowth/go-wisebot/config"
	"github.com/WiseGrowth/go-wisebot/logger"
	"github.com/WiseGrowth/go-wisebot/rasp"
	"github.com/WiseGrowth/wisebot-operator/command"
	"github.com/WiseGrowth/wisebot-operator/daemon"
	"github.com/WiseGrowth/wisebot-operator/git"
	"github.com/WiseGrowth/wisebot-operator/iot"
	homedir "github.com/mitchellh/go-homedir"
)

var (
	operatorVersion string
)

const (
	wisebotCoreServiceName = "wisebot-core"
	wisebotCoreRepoPath    = "~/wisebot-core"
	wisebotCoreRepoRemote  = "git@github.com:wisegrowth/test.git"

	wisebotBleServiceName = "wisebot-ble"
	wisebotBleRepoPath    = "~/wisebot-ble"
	wisebotBleRepoRemote  = "git@github.com:wisegrowth/wisebot-ble.git"

	wisebotLedDaemonName       = "led"
	wisebotLedDaemonRepoPath   = "~/wisebot-led-indicator"
	wisebotLedDaemonRepoRemote = "git@github.com:wisegrowth/wisebot-led-indicator.git"

	wisebotConfigPath = "~/.config/wisebot/config.json"
	wisebotLogPath    = "~/.wisebot/logs/operator.log"
)

var (
	wisebotCoreRepoExpandedPath      string
	wisebotBleRepoExpandedPath       string
	wisebotLedDaemonRepoExpandedPath string

	wisebotConfig *config.Config
	wisebotLogger io.WriteCloser

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

	wisebotBleRepoExpandedPath, err = homedir.Expand(wisebotBleRepoPath)
	check(err)

	wisebotLedDaemonRepoExpandedPath, err = homedir.Expand(wisebotLedDaemonRepoPath)
	check(err)

	// ----- Load wisebot config
	wisebotConfig, err = config.LoadConfig(wisebotConfigPath)
	check(err)

	healthzPublishableTopic = fmt.Sprintf("/operator/%s/healthz", wisebotConfig.WisebotID)

	wisebotLogger, err = newFile(wisebotLogPath)
	check(err)
}

func main() {
	defer wisebotLogger.Close()
	check(logger.Init(wisebotLogger, wisebotConfig.WisebotID, wisebotConfig.SentryDSN))

	log := logger.GetLogger().WithField("version", operatorVersion)
	log.Info("Starting")

	// ----- Initialize git repos
	ledDaemonRepo := git.NewRepo(
		wisebotLedDaemonRepoExpandedPath,
		wisebotLedDaemonRepoRemote,
		git.YarnInstallHook,
	)

	coreRepo := git.NewRepo(
		wisebotCoreRepoExpandedPath,
		wisebotCoreRepoRemote,
		git.YarnInstallHook,
	)

	bleRepo := git.NewRepo(
		wisebotBleRepoExpandedPath,
		wisebotBleRepoRemote,
		git.YarnInstallHook,
	)

	// ----- Initialize daemons
	if runtime.GOOS != "darwin" {
		d, err := daemon.NewDaemon(wisebotLedDaemonName, ledDaemonRepo)
		check(err)
		daemonStore.Save(d)
	}

	// ----- Initialize commands
	wisebotCoreCommand := command.NewCommand(
		"node",
		wisebotCoreRepoExpandedPath+"/build/app/index.js",
	)
	wisebotBleCommand := command.NewCommand(
		"node",
		wisebotBleRepoExpandedPath+"/build/app/index.js",
	)

	// ----- Initialize MQTT client
	cert, err := wisebotConfig.GetTLSCertificate()
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
	services.Save(wisebotCoreServiceName, wisebotCoreCommand, coreRepo)
	services.Save(wisebotBleServiceName, wisebotBleCommand, bleRepo)

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
