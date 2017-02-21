package rasp

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/WiseGrowth/wisebot-operator/logger"
)

const (
	apModeESSID = "Wisebot"

	apInterface   = "wlan0"
	wifiInterface = "wlan1"
)

var (
	onOSX bool
)

func init() {
	onOSX = (runtime.GOOS == "darwin")
}

// IsAPModeActive check if the `apInterface` interface is in AP Mode, returns
// true if is enabled, or false if it isn't.
func IsAPModeActive() (bool, error) {
	if onOSX {
		return false, nil
	}

	out, err := exec.Command("iwconfig", apInterface).Output()
	if err != nil {
		return false, err
	}

	search := fmt.Sprintf("ESSID:\"%s\"", apModeESSID)
	onmode := bytes.Contains(out, []byte(search))

	return onmode, nil
}

// Wifi errors
var (
	ErrNoWifi = errors.New("Could not connect to network")
)

var (
	qualityRegexGroup     = regexp.MustCompile("Quality=(\\d+)/100")
	signalLevelRegexGroup = regexp.MustCompile("Signal\\s+level=(\\d+)/100")
)

// Network represents an available wifi network.
type Network struct {
	Address     string `json:"address"`
	ESSID       string `json:"essid"`
	Encryption  string `json:"encryption"`
	Quality     int    `json:"quality"`
	SignalLevel int    `json:"signal_level"`
	Password    string `json:"password,omitempty"`
}

// IsWPA returns true if the encryption is WPA.
func (n *Network) IsWPA() bool {
	return strings.Contains(n.Encryption, "WPA")
}

// AvailableNetworks return an array of unique available wifi networks.
// if there is more than one network with the same ESSID, it just considers the
// one with the higher signal level value.
func AvailableNetworks() ([]*Network, error) {
	out, err := exec.Command("sudo", "iwlist", apInterface, "scan").Output()
	if err != nil {
		return nil, err
	}

	networks := map[string]*Network{}

	cells := strings.Split(string(out), "Cell")
	for _, cell := range cells {
		lines := strings.Split(cell, "\n")

		n := new(Network)
		for _, line := range lines {
			if strings.Contains(line, "Address") {
				i := strings.Index(line, ":")
				n.Address = strings.TrimSpace(line[i+1:])
			}

			if strings.Contains(line, "ESSID") {
				i := strings.Index(line, ":")
				n.ESSID = strings.Trim(line[i+1:], "\"")
			}

			if strings.Contains(line, "IE: IEEE") {
				i := strings.Index(line, "/")
				n.Encryption = strings.TrimSpace(line[i+1:])
			}

			if strings.Contains(line, "Quality") {
				match := qualityRegexGroup.FindStringSubmatch(line)
				if len(match) > 0 {
					val, err := strconv.Atoi(match[1])
					if err != nil {
						return nil, err
					}
					n.Quality = val
				}
				match = signalLevelRegexGroup.FindStringSubmatch(line)
				if len(match) > 0 {
					val, err := strconv.Atoi(match[1])
					if err != nil {
						return nil, err
					}
					n.SignalLevel = val
				}
			}
		}

		if len(n.ESSID) != 0 {
			if savedNetwork, ok := networks[n.ESSID]; ok {
				if savedNetwork.SignalLevel < n.SignalLevel {
					networks[n.ESSID] = n
				}
			} else {
				networks[n.ESSID] = n
			}
		}
	}

	var res []*Network
	for _, n := range networks {
		res = append(res, n)
	}

	return res, nil
}

// SetupWifi configures the raspberry pi wifi network.
func SetupWifi(n *Network) error {
	log := logger.GetLogger().WithFields(logrus.Fields{
		"file":     interfacesPath,
		"function": "SetupWifi",
	})

	log.Debug("Open file")
	f, err := os.OpenFile(interfacesPath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	if err := f.Truncate(0); err != nil {
		return err
	}

	log.Debug("Write config")
	if err := wifiConfigTemplate.Execute(f, n); err != nil {
		f.Close()
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	log.Debug("Restarting Network")
	if err := restartNetworkInterface(wifiInterface); err != nil {
		return ErrNoWifi
	}

	log.Debug("Checking connection with ping")
	connected, err := IsConnected()
	if err != nil {
		return err
	}

	if !connected {
		return ErrNoWifi
	}

	return nil
}

// IsConnected executes a ping command in order to check wether the device is
// connected to the network.
func IsConnected() (bool, error) {
	if onOSX {
		return true, nil
	}

	ping := exec.Command("ping", "-t", "20", "-w", "1", "8.8.8.8")

	if err := ping.Run(); err != nil {
		// Ignore exit errors
		if _, ok := (err).(*exec.ExitError); ok {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// ActivateAPMode restarts the raspberry `apInterface` interface, hostapd
// and isc-dhcp-server services. If the interface is already in AP Mode, it just
// restarts the isc-dhcp-server.
func ActivateAPMode() error {
	log := logger.GetLogger().WithField("function", "ActivateAPMode")
	iscservercmd := exec.Command("sudo", "service", "isc-dhcp-server", "restart")

	if isActive, _ := IsAPModeActive(); isActive {
		log.Debug("sudo service isc-dhcp-server restart")
		return iscservercmd.Run()
	}

	if err := restartNetworkInterface(apInterface); err != nil {
		return err
	}

	log.Debug("Running: sudo service hostapd restart")
	hostapdcmd := exec.Command("sudo", "service", "hostapd", "restart")

	if err := hostapdcmd.Run(); err != nil {
		return err
	}

	log.Debug("sudo service isc-dhcp-server restart")
	return iscservercmd.Run()
}

// DeactivateAPMode deactivates hostapd service
func DeactivateAPMode() error {
	log := logger.GetLogger().WithField("function", "DeactivateAPMode")
	log.Debug("Running: sudo service hostapd stop")
	hostapdcmd := exec.Command("sudo", "service", "hostapd", "stop")
	return hostapdcmd.Run()
}

func restartNetworkInterface(netInterface string) error {
	log := logger.GetLogger().WithField("function", "restartNetworkInterface")

	log.Debug("Running: sudo ifdown " + netInterface)
	ifdown := exec.Command("sudo", "ifdown", netInterface)
	ifdown.Run()

	log.Debug("Running: sudo ifup " + netInterface)
	ifup := exec.Command("sudo", "ifup", netInterface)

	return ifup.Run()
}

// waitForNetwork just performs a ping command to google's DNS server to check
// if the network is up or down.
// The command will execute for 3 minutes and it sleeps 4 seconds before
// trying again if the ping command fails.
// The ping command ignores the exec.ExitError errors since this tell us that
// the network is up or down, all other errors are returned since are
// unexpected errors.
// It returns a nil error if the device is connected to the internet.
func waitForNetwork() error {
	sleepDuration := time.Second * 4
	log := logger.GetLogger().WithField("function", "waitForNetwork")

	for {
		ping := exec.Command("ping", "-w", "1", "8.8.8.8")

		select {
		case <-time.After(time.Minute * 3):
			return ErrNoWifi
		default:
			log.Debug("Pinging")
			if err := ping.Run(); err != nil {
				// Ignore exit errors
				if _, ok := (err).(*exec.ExitError); !ok {
					return err
				}
				break // exit error
			}

			return nil
		}

		log.Debug("Sleep 4 seconds")
		time.Sleep(sleepDuration)
	}
}
