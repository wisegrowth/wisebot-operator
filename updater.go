package main

import (
	"bytes"
	"crypto"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/WiseGrowth/go-wisebot/logger"
	update "github.com/inconshreveable/go-update"
)

// Updater is in charge to update the operator
type Updater struct {
	uploading bool
	baseURL   string
	version   string
}

// NewUpdate constructor method for Updater struct
func NewUpdate(baseURL string, currentVersion string) *Updater {
	return &Updater{uploading: false, baseURL: baseURL, version: currentVersion}
}

// Update method used for update this operator service based of param version
func (up *Updater) Update(newVersion string) error {
	//TODO: validate newVersion format, eg. 1.2.3
	if newVersion == up.version {
		err := errors.New("new version is the same that currently exists in the service")
		return err
	}

	up.uploading = true

	var urlChecksum = up.baseURL + newVersion + ".checksum"
	var urlBin = up.baseURL + newVersion

	log := logger.GetLogger()
	log.Info("New Version: ", newVersion)
	log.Info("URL Checksum: ", urlChecksum)
	log.Info("URL Bin: ", urlBin)

	//define http client
	client := http.Client{}

	//get checksum file
	log.Info("[Download Checksum]")
	respChecksum, err := client.Get(urlChecksum)
	if err != nil {
		return err
	}
	if respChecksum.StatusCode != 200 {
		msg := "Checksum file not available to " + newVersion
		err := errors.New(msg)
		return err
	}
	log.Info("[Download Checksum] ready...")

	//get checksum value
	buf := new(bytes.Buffer)
	buf.ReadFrom(respChecksum.Body)
	checksum := buf.String()
	checksum = strings.TrimSpace(checksum)
	respChecksum.Body.Close()

	//get binary
	log.Info("[Download Binary]")
	respBin, err := client.Get(urlBin)
	if err != nil {
		return err
	}
	if respBin.StatusCode != 200 {
		msg := "Checksum file not available to " + newVersion
		err := errors.New(msg)
		return err
	}
	log.Info("[Download Binary] ready...")

	//update binary
	log.Info("[Upload Version]")
	err = up.updateWithChecksum(respBin.Body, checksum)
	if err != nil {
		log.Error(err)
	}
	respBin.Body.Close()
	log.Info("[Upload Version] Ready")

	up.uploading = false
	return err
}

func (up *Updater) updateWithChecksum(binary io.Reader, hexChecksum string) error {
	checksum, err := hex.DecodeString(hexChecksum)
	if err != nil {
		return err
	}

	err = update.Apply(binary, update.Options{
		Hash:     crypto.SHA256,
		Checksum: checksum,
	})

	return err
}
