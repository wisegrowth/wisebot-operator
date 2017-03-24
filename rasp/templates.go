package rasp

import "html/template"

const (
	wpaSupplicantPath        = "/etc/wpa_supplicant/wpa_supplicant.conf"
	wpaSupplicantConfigTempl = `country=GB
ctrl_interface=DIR=/var/run/wpa_supplicant GROUP=netdev
update_config=1
network={
    ssid="[[.ESSID]]"
    scan_ssid=1
    key_mgmt=[[if .IsWPA ]]WPA-PSK[[else]]NONE[[end]]
    [[if .Password]][[if .IsWPA ]]psk[[else]]wep_key0[[end]]="[[.Password]]"[[end]]
    priority=1
	}

network={
    ssid="wisegrowth-420"
    scan_ssid=1
    key_mgmt=WPA-PSK
    psk="wisegrowth-420"
    priority=5
	}

`
)

var (
	wpaSupplicantConfigTemplate = template.Must(template.New("wpaSupplicantConfigTemplate").Delims("[[", "]]").Parse(wpaSupplicantConfigTempl))
)
