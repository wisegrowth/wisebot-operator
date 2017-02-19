package rasp

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/WiseGrowth/wisebot-operator/logger"
)

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
	Password    string `json:"-"`
}

// IsWPA returns true if the encryption is WPA.
func (n *Network) IsWPA() bool {
	return strings.Contains(n.Encryption, "WPA")
}

// AvailableNetworks return an array of unique available wifi networks.
// if there is more than one network with the same ESSID, it just considers the
// one with the higher signal level value.
func AvailableNetworks() ([]*Network, error) {
	out, err := exec.Command("sudo", "iwlist", "wlan0", "scan").Output()
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
				i := strings.Index(line, "/")
				n.Encryption = strings.TrimSpace(line[i+1:])
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

func prepareInterfaceFileForAPMode() error {
	log := logger.GetLogger().WithField("function", "prepareInterfaceFileForAPMode")

	f, err := os.OpenFile(interfacesPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	data := []byte(interfaceAPTemplateString)
	log.Debug("Writing interface file")
	for t := 0; t < len(data); t++ {
		n, err := f.Write(data)
		t += n

		if err != nil {
			return err
		}
	}

	if err := f.Sync(); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return nil
}

// SetAPMode sets the raspberry wlan0 interface as an Access Point.
func SetAPMode() error {
	if err := prepareInterfaceFileForAPMode(); err != nil {
		return err
	}

	authoritative := true
	if err := updateDHCPConfig(authoritative); err != nil {
		return err
	}

	if err := restartWifi(); err != nil {
		return err
	}

	log := logger.GetLogger().WithField("function", "SetAPMode")
	log.Debug("Running: sudo service hostapd restart")
	hostapdcmd := exec.Command("sudo", "service", "hostapd", "restart")

	if err := hostapdcmd.Run(); err != nil {
		return err
	}

	log.Debug("sudo service isc-dhcp-server restart")
	iscservercmd := exec.Command("sudo", "service", "isc-dhcp-server", "restart")

	if err := iscservercmd.Run(); err != nil {
		return err
	}

	return nil
}

func restartWifi() error {
	log := logger.GetLogger().WithField("function", "restartWifi")

	log.Debug("Running: sudo ifdown wlan0")
	ifdown := exec.Command("sudo", "ifdown", "wlan0")
	if err := ifdown.Run(); err != nil {
		return err
	}

	log.Debug("Running: sudo ifup wlan0")
	ifup := exec.Command("sudo", "ifup", "wlan0")

	return ifup.Run()
}

func updateDHCPConfig(authoritative bool) error {
	log := logger.GetLogger().WithField("function", "updateDHCPConfig")

	log.Debug("Open file")
	file, err := os.OpenFile(dhcpdConfigPath, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	defer file.Sync()

	data := &struct {
		Authoritative bool
	}{authoritative}

	log.Debug("Write config")
	if err := dhcpdConfigTemplate.Execute(file, data); err != nil {
		return err
	}

	return nil
}

// SetupWifi configures the raspberry pi wifi network.
func SetupWifi(n *Network) error {
	log := logger.GetLogger().WithFields(logrus.Fields{
		"file":     interfacesPath,
		"function": "SetupWifi",
	})

	log.Debug("Open file")
	file, err := os.OpenFile(interfacesPath, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	log.Debug("Write config")
	if err := wifiConfigTemplate.Execute(file, n); err != nil {
		file.Close()
		return err
	}

	if err := file.Sync(); err != nil {
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	if err := restartWifi(); err != nil {
		return err
	}

	if err := waitForNetwork(); err != nil {
		return err
	}

	authoritative := false
	return updateDHCPConfig(authoritative)
}

// IsConnected executes a ping command in order to check wether the device is
// connected to the network.
func IsConnected() (bool, error) {
	ping := exec.Command("ping", "-t", "20", "-w", "1", "8.8.8.8")

	if err := ping.Run(); err != nil {
		// Ignore exit errors
		if _, ok := (err).(*exec.ExitError); !ok {
			return false, nil
		}
		return false, err // exit error
	}

	return true, nil
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
