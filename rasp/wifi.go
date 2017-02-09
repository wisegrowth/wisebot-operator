package rasp

import (
	"errors"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/WiseGrowth/operator/command"
)

// Network represents an available wifi network.
type Network struct {
	Address    string `json:"address"`
	ESSID      string `json:"essid"`
	Encryption string `json:"encryption"`
	Password   string `json:"-"`
}

// IsWPA returns true if the encryption is
// wpa.
func (n *Network) IsWPA() bool {
	return strings.Contains(n.Encryption, "WPA")
}

// AvailableNetworks return an array of available
// wifi networks.
func AvailableNetworks() ([]*Network, error) {
	out, err := exec.Command("sudo", "iwlist", "wlan0", "scan").Output()
	if err != nil {
		fmt.Println("error comando", err)
		return nil, err
	}

	var networks []*Network
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
		}

		if len(n.ESSID) != 0 {
			networks = append(networks, n)
		}
	}

	return networks, nil
}

const (
	wpaSupplicantPath = "/etc/wpa_supplicant/wpa_supplicant.conf"
	wifiConfigTmpl    = `country=GB
ctrl_interface=DIR=/var/run/wpa_supplicant GROUP=netdev
update_config=1

network={
    ssid="[[.ESSID]]"
    scan_ssid=1
    key_mgmt=[[if .IsWPA ]]WPA-PSK[[else]]NONE[[end]]
    [[if .Password]][[if .IsWPA ]]psk[[else]]wep_key0[[end]]="[[.Password]]"[[end]]
}
`
)

// SetupWifi configures the raspberry pi wifi network.
func SetupWifi(n *Network) error {
	file, err := os.OpenFile(wpaSupplicantPath, os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}

	defer file.Close()
	t := template.Must(template.New("wifiConfigWpa").Delims("[[", "]]").Parse(wifiConfigTmpl))

	if err := t.Execute(file, n); err != nil {
		return err
	}

	// If we use the os.Stdout as the log file, the command sometimes fail
	// perhaps we should look into it.
	cmd := command.NewCommand(nil, "wpa_cli", "reconfigure")
	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	return waitForNetwork()
}

// IsConnected executes a ping command in order to check
// wether the device is connected to the network.
func IsConnected() (bool, error) {
	ping := command.NewCommand(nil, "ping", "-w", "1", "8.8.8.8")
	timeout := time.NewTimer(15 * time.Second)

	select {
	case <-timeout.C:
		return false, errors.New("Could not connect to the wifi")
	default:
		if err := ping.Start(); err != nil {
			return false, err
		}
		if err := ping.Wait(); err != nil {
			// Ignore exit errors
			if _, ok := (err).(*exec.ExitError); !ok {
				return false, err
			}
		}

		if ping.Success() {
			return true, nil
		}
	}

	return false, nil
}

// waitForNetwork just perform a ping command to google's DNS server
// to check if the network is up or down.
// The command will execute for 3 minutes and it sleeps 4 seconds before
// trying again if the ping command fails.
// The ping command ignores the exec.ExitError errors since this tell us that
// the network is up or down, all other errors are returned since
// are unexpected errors.
func waitForNetwork() error {
	timeout := time.NewTimer(time.Minute * 3)
	sleepDuration := time.Second * 4

	for {
		ping := command.NewCommand(nil, "ping", "-w", "1", "8.8.8.8")

		select {
		case <-timeout.C:
			return errors.New("Could not connect to the wifi")
		default:
			if err := ping.Start(); err != nil {
				return err
			}
			if err := ping.Wait(); err != nil {
				// Ignore exit errors
				if _, ok := (err).(*exec.ExitError); !ok {
					return err
				}
			}

			if ping.Success() {
				return nil
			}

			time.Sleep(sleepDuration)
		}
	}
}
