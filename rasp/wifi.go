package rasp

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/WiseGrowth/wisebot-operator/logger"
)

const (
	wifiInterface = "wlan0"
)

var (
	onOSX bool
)

func init() {
	onOSX = (runtime.GOOS == "darwin")
}

// Wifi errors
var (
	ErrNoWifi = errors.New("Could not connect to network")
)

var (
	wpaSupplicantESSIDRegexGroup = regexp.MustCompile("ssid=\"([^\"]*)\"")
	qualityRegexGroup            = regexp.MustCompile("Quality=(\\d+)/100")
	signalLevelRegexGroup        = regexp.MustCompile("Signal\\s+level=(\\d+)/100")
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

// CurrentConfiguredNetworkESSID returns current configured network essid on the
// wpa_supplicant.conf file.
func CurrentConfiguredNetworkESSID() (string, error) {
	wpacfg, err := ioutil.ReadFile(wpaSupplicantPath)
	if err != nil {
		return "", err
	}

	matches := wpaSupplicantESSIDRegexGroup.FindSubmatch(wpacfg)
	if len(matches) != 2 {
		return "", nil
	}

	return string(matches[1]), nil
}

// AvailableNetworks return an array of unique available wifi networks.
// if there is more than one network with the same ESSID, it just considers the
// one with the higher signal level value.
func AvailableNetworks() ([]*Network, error) {
	out, err := exec.Command("sudo", "iwlist", wifiInterface, "scan").Output()
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

func openFileAndTruncate(name string) (*os.File, error) {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}

	if err := f.Truncate(0); err != nil {
		return nil, err
	}

	return f, nil
}

// SetupWifi configures the raspberry pi wifi network.
func SetupWifi(n *Network) error {
	log := logger.GetLogger().WithField("function", "SetupWifi")

	log.Debug("Updating wpa_supplicant.conf file and reconfigure wpa_supplicant")
	if err := reconfigureWPASupplicant(n); err != nil {
		return err
	}

	log.Debug("Restarting interface " + wifiInterface)
	if err := restartInterface(wifiInterface); err != nil {
		return err
	}

	log.Debug("Checking connection with ping")
	return waitForNetwork()
}

// restartInterface just runs an `sudo ifdown` and `sudo ifup` with the
// indicated interface.
func restartInterface(networkInterface string) error {
	if err := exec.Command("sudo", "ifdown", networkInterface).Run(); err != nil {
		return err
	}

	return exec.Command("sudo", "ifup", networkInterface).Run()
}

func reconfigureWPASupplicant(n *Network) error {
	f, err := openFileAndTruncate(wpaSupplicantPath)
	if err != nil {
		return err
	}

	if err := wpaSupplicantConfigTemplate.Execute(f, n); err != nil {
		f.Close()
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return exec.Command("wpa_cli", "reconfigure").Run()
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

// waitForNetwork just performs a ping command to google's DNS server to check
// if the network is up or down.
// The command will execute for 3 minutes and it sleeps 4 seconds before
// trying again if the ping command fails.
// The ping command ignores the exec.ExitError errors since this tell us that
// the network is up or down, all other errors are returned since are
// unexpected errors.
// It returns a nil error if the device is connected to the internet.
func waitForNetwork() error {
	sleepDuration := time.Second * 1
	log := logger.GetLogger().WithField("function", "waitForNetwork")

	tries := 0
	for tries < 7 {
		ping := exec.Command("ping", "-w", "1", "8.8.8.8")

		log.Debug("Pinging")
		if err := ping.Run(); err != nil {
			// Ignore exit errors
			if _, ok := (err).(*exec.ExitError); !ok {
				return err
			}
		} else {
			return nil
		}

		log.Debug("Sleep 1 second")
		time.Sleep(sleepDuration)
		tries++
	}
	return ErrNoWifi
}
