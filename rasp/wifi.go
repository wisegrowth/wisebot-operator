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
	WPA        bool   `json:"-"`
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
				essid := line[i+1:]               // surrounded with quotes
				n.ESSID = essid[1 : len(essid)-1] // without quotes
			}

			if strings.Contains(line, "IE: IEEE") {
				i := strings.Index(line, "/")
				n.Encryption = line[i+1:]

				n.WPA = strings.Contains(n.Encryption, "WPA")
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
    key_mgmt=[[if .WPA]]WPA-PSK[[else]]NONE[[end]]
    [[if .Password]][[if .WPA]]psk[[else]]wep_key0[[end]]="[[.Password]]"[[end]]

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
