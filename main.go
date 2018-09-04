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
	operatorVersion                   string
	wisebotCoreRepoBranchName         string
	wisebotBleRepoBranchName          string
	wisebotScriptRepoBranchName       string
	wisebotButtonRepoBranchName       string
	wisebotStorageRepoBranchName      string
	wisebotLedDaemonRepoBranchName    string
	wisebotTunnelDaemonRepoBranchName string
)

const (
	wisebotCoreServiceName = "wisebot-core"
	wisebotCoreRepoPath    = "~/wisebot-core"
	wisebotCoreRepoRemote  = "git@github.com:wisegrowth/wisebot-core.git"

	wisebotBleServiceName = "wisebot-ble"
	wisebotBleRepoPath    = "~/wisebot-ble"
	wisebotBleRepoRemote  = "git@github.com:wisegrowth/wisebot-ble.git"

	wisebotScriptServiceName = "wisebot-script"
	wisebotScriptRepoPath    = "~/wisebot-script"
	wisebotScriptRepoRemote  = "git@github.com:wisegrowth/wisebot-script.git"

	wisebotButtonServiceName = "wisebot-button"
	wisebotButtonRepoPath    = "~/wisebot-button"
	wisebotButtonRepoRemote  = "git@github.com:wisegrowth/wisebot-button.git"

	wisebotLedDaemonName       = "led"
	wisebotLedDaemonRepoPath   = "~/wisebot-led-indicator"
	wisebotLedDaemonRepoRemote = "git@github.com:wisegrowth/wisebot-led-indicator.git"

	wisebotSSHTunnelDaemonName     = "ssh-tunnel"
	wisebotStorageTunnelDaemonName = "storage-tunnel"
	wisebotTunnelDaemonRepoPath    = "~/wisebot-tunnel"
	wisebotTunnelDaemonRepoRemote  = "git@github.com:wisegrowth/wisebot-tunnel.git"

	wisebotStorageServiceName = "wisebot-storage"
	wisebotStorageRepoPath    = "~/wisebot-storage"
	wisebotStorageRepoRemote  = "git@github.com:wisegrowth/wisebot-storage.git"

	//TODO: should implement support to use NewDaemon without code's repository

	wisebotConfigPath = "~/.config/wisebot/config.json"
	wisebotLogPath    = "~/.wisebot/logs/operator.log"

	defaultBranchName = "master"
)

var (
	version                             string
	baseURL                             string
	wisebotCoreRepoExpandedPath         string
	wisebotBleRepoExpandedPath          string
	wisebotScriptRepoExpandedPath       string
	wisebotButtonRepoExpandedPath       string
	wisebotLedDaemonRepoExpandedPath    string
	wisebotTunnelDaemonRepoExpandedPath string
	wisebotStorageRepoExpandedPath      string

	wisebotConfig *config.Config
	wisebotLogger io.WriteCloser

	healthzPublishableTopic string

	httpServer     *http.Server
	processManager *ProcessManager
	daemonStore    *daemon.Store
)

func init() {
	var err error

	version = "1.7.0"

	// ----- Load wisebot config
	wisebotConfig, err = config.LoadConfig(wisebotConfigPath)
	check(err)

	processManager = new(ProcessManager)
	daemonStore = new(daemon.Store)

	wisebotCoreRepoExpandedPath, err = homedir.Expand(wisebotCoreRepoPath)
	check(err)

	wisebotBleRepoExpandedPath, err = homedir.Expand(wisebotBleRepoPath)
	check(err)
	wisebotLedDaemonRepoExpandedPath, err = homedir.Expand(wisebotLedDaemonRepoPath)
	check(err)

	wisebotScriptRepoExpandedPath, err = homedir.Expand(wisebotScriptRepoPath)
	check(err)

	wisebotButtonRepoExpandedPath, err = homedir.Expand(wisebotButtonRepoPath)
	check(err)

	wisebotTunnelDaemonRepoExpandedPath, err = homedir.Expand(wisebotTunnelDaemonRepoPath)
	check(err)

	wisebotStorageRepoExpandedPath, err = homedir.Expand(wisebotStorageRepoPath)
	check(err)

	healthzPublishableTopic = fmt.Sprintf("/operator/%s/healthz", wisebotConfig.WisebotID)

	wisebotLogger, err = newFile(wisebotLogPath)
	check(err)
}

func main() {
	defer wisebotLogger.Close()
	check(logger.Init(wisebotLogger, wisebotConfig.WisebotID, wisebotConfig.SentryDSN))

	// ----- Validate existence of repositories branch names
	if len(wisebotConfig.ScriptBranch) == 0 {
		wisebotScriptRepoBranchName = defaultBranchName
	} else {
		wisebotScriptRepoBranchName = wisebotConfig.ScriptBranch
	}

	if len(wisebotConfig.LedBranch) == 0 {
		wisebotLedDaemonRepoBranchName = defaultBranchName
	} else {
		wisebotLedDaemonRepoBranchName = wisebotConfig.LedBranch
	}

	if len(wisebotConfig.CoreBranch) == 0 {
		wisebotCoreRepoBranchName = defaultBranchName
	} else {
		wisebotCoreRepoBranchName = wisebotConfig.CoreBranch
	}

	if len(wisebotConfig.BleBranch) == 0 {
		wisebotBleRepoBranchName = defaultBranchName
	} else {
		wisebotBleRepoBranchName = wisebotConfig.BleBranch
	}

	if len(wisebotConfig.ButtonBranch) == 0 {
		wisebotButtonRepoBranchName = defaultBranchName
	} else {
		wisebotButtonRepoBranchName = wisebotConfig.ButtonBranch
	}

	if len(wisebotConfig.TunnelBranch) == 0 {
		wisebotTunnelDaemonRepoBranchName = defaultBranchName
	} else {
		wisebotTunnelDaemonRepoBranchName = wisebotConfig.TunnelBranch
	}

	if len(wisebotConfig.StorageBranch) == 0 {
		wisebotStorageRepoBranchName = defaultBranchName
	} else {
		wisebotStorageRepoBranchName = wisebotConfig.StorageBranch
	}

	log := logger.GetLogger().WithField("version", operatorVersion)
	log.Info("Starting")

	// ----- Initialize git repos
	scriptRepo := git.NewRepo(
		wisebotScriptRepoExpandedPath,
		wisebotScriptRepoRemote,
		wisebotScriptRepoBranchName,
	)

	ledDaemonRepo := git.NewRepo(
		wisebotLedDaemonRepoExpandedPath,
		wisebotLedDaemonRepoRemote,
		wisebotLedDaemonRepoBranchName,
		git.YarnInstallHook,
	)

	tunnelDaemonRepo := git.NewRepo(
		wisebotTunnelDaemonRepoExpandedPath,
		wisebotTunnelDaemonRepoRemote,
		wisebotTunnelDaemonRepoBranchName,
		git.YarnInstallHook,
	)

	coreRepo := git.NewRepo(
		wisebotCoreRepoExpandedPath,
		wisebotCoreRepoRemote,
		wisebotCoreRepoBranchName,
		git.YarnInstallHook,
	)

	bleRepo := git.NewRepo(
		wisebotBleRepoExpandedPath,
		wisebotBleRepoRemote,
		wisebotBleRepoBranchName,
		git.YarnInstallHook,
	)

	buttonRepo := git.NewRepo(
		wisebotButtonRepoExpandedPath,
		wisebotButtonRepoRemote,
		wisebotButtonRepoBranchName,
		git.NpmInstallHook,
	)

	storageRepo := git.NewRepo(
		wisebotStorageRepoExpandedPath,
		wisebotStorageRepoRemote,
		wisebotStorageRepoBranchName,
	)

	// ----- Initialize daemons
	if runtime.GOOS != "darwin" {
		d, err := daemon.NewDaemon(wisebotLedDaemonName, ledDaemonRepo)
		check(err)
		daemonStore.Save(d)

		d, err = daemon.NewDaemon(wisebotSSHTunnelDaemonName, tunnelDaemonRepo)
		check(err)
		daemonStore.Save(d)

		d, err = daemon.NewDaemon(wisebotStorageTunnelDaemonName, tunnelDaemonRepo)
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

	wisebotScriptCommand := command.NewCommand(
		"sudo",
		wisebotScriptRepoExpandedPath+"/wisebot-script",
	)

	wisebotButtonCommand := command.NewCommand(
		"sudo",
		"node",
		wisebotButtonRepoExpandedPath+"/index.js",
	)

	wisebotStorageCommand := command.NewCommand(
		wisebotStorageRepoExpandedPath + "/wisebot-storage",
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
	services.Save(wisebotScriptServiceName, wisebotScriptCommand, scriptRepo)
	services.Save(wisebotButtonServiceName, wisebotButtonCommand, buttonRepo)
	services.Save(wisebotStorageServiceName, wisebotStorageCommand, storageRepo)

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
	updateSourceCode := isConnected
	check(processManager.KickOffServices(updateSourceCode))
	check(daemonStore.Bootstrap(updateSourceCode))
	if isConnected {
		check(processManager.KickOffMQTTClient())
	} else {
		tick := time.NewTicker(30 * time.Second)
		go func() {
			defer tick.Stop()
			for range tick.C {
				isConnected, _ := rasp.IsConnected()
				if isConnected {
					if err := processManager.KickOffMQTTClient(); err != nil {
						log.Error(err)
						quit <- struct{}{}
						return
					}
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
		s := <-c
		logger.GetLogger().WithField("signal", s.String()).Debug("Signal received")
		quit <- struct{}{}
	}()
}

func gracefullShutdown() {
	log := logger.GetLogger()
	log.Debug("Gracefully shutdown")
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
