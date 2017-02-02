package rasp

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"strings"
)

// Network represents an available wifi network
type Network struct {
	Address    string `json:"address"`
	ESSID      string `json:"essid"`
	Encryption string `json:"encryption"`
	Password   string `json:"-"`
}

// IsWPA returns true if the encryption is
// wpa
func (n *Network) IsWPA() bool {
	return strings.Contains(n.Encryption, "WPA")
}

// AvailableNetworks return an array of available
// wifi networks
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
				n.Address = line[i+2:]
			}

			if strings.Contains(line, "ESSID") {
				i := strings.Index(line, ":")
				essid := string(line[i+1])
				n.ESSID = strings.Trim(essid, "\"")
			}

			if strings.Contains(line, "IE: IEEE") {
				i := strings.Index(line, "/")
				n.Encryption = line[i+1:]
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

// SetupWifi configures the raspberry pi wifi network
func SetupWifi(n *Network) error {
	file, err := os.OpenFile(wpaSupplicantPath, os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}

	defer file.Close()
	t := template.Must(template.New("wifiConfigWpa").Delims("[[", "]]").Parse(wifiConfigTmpl))

	return t.Execute(file, n)
}
