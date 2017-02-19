package rasp

import "html/template"

const (
	interfacesPath            = "/etc/network/interfaces"
	interfaceAPTemplateString = `# [Generated by WisebotOperator]

auto lo

iface lo inet loopback
iface eth0 inet dhcp

allow-hotplug wlan0
auto wlan0
iface wlan0 inet static
address 192.168.42.1
netmask 255.255.255.0




`
	interfaceWifiTempl = `# [Generated by WisebotOperator]

auto lo

iface lo inet loopback
iface eth0 inet dhcp

allow-hotplug wlan0
auto wlan0
iface wlan0 inet dhcp
    wpa-ssid "[[.ESSID]]"
    wpa-psk "[[.Password]]"




`

	dhcpdConfigPath = "/etc/dhcp/dhcpd.conf"
	dhcpdConfgTempl = `# [Generated by WisebotOperator]

ddns-update-style none;

default-lease-time 600;
max-lease-time 7200;

[[if .Authoritative ]][[else]]#[[end]]authoritative;

log-facility local7;

subnet 192.168.42.0 netmask 255.255.255.0 {
    range 192.168.42.10 192.168.42.50;
    option broadcast-address 192.168.42.255;
    option routers 192.168.42.1;
    default-lease-time 600;
    max-lease-time 7200;
    option domain-name wisebot;
}
`
)

var (
	wifiConfigTemplate  = template.Must(template.New("wifiTempl").Delims("[[", "]]").Parse(interfaceWifiTempl))
	dhcpdConfigTemplate = template.Must(template.New("dhcpTempl").Delims("[[", "]]").Parse(dhcpdConfgTempl))
)
