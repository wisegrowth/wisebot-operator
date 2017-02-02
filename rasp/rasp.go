package rasp

import (
	"io/ioutil"
	"os/exec"
	"regexp"
)

const (
	gpioDir = "/sys/class/gpio"
)

var (
	gpioFileName = regexp.MustCompile("\\Agpio(\\d{1,2})\\z")
)

// TurnOffPins shuts down all GPIO pins,
// if all pins are turned off it returns a nil error
func TurnOffPins() error {
	pins, err := availablePins()
	if err != nil {
		return err
	}

	if len(pins) < 1 {
		return nil
	}

	for _, pin := range pins {
		cmd := exec.Command("gpio-admin", "unexport", pin)

		if err := cmd.Start(); err != nil {
			return err
		}

		if err := cmd.Wait(); err != nil {
			return err
		}
	}

	return nil
}

// availablePins look for exported pins and
func availablePins() ([]string, error) {
	files, err := ioutil.ReadDir(gpioDir)
	if err != nil {
		return nil, err
	}

	var pins []string

	for _, f := range files {
		name := f.Name()
		matches := gpioFileName.FindStringSubmatch(name)

		if len(matches) > 0 {
			pin := matches[1]
			pins = append(pins, pin)
		}
	}

	return pins, nil
}
