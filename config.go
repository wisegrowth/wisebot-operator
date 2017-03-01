package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	homedir "github.com/mitchellh/go-homedir"
)

type config struct {
	WisebotID    string `json:"id"`
	Certificates struct {
		PrivateKey  string `json:"private_key"`
		Certificate string `json:"certificate"`
	} `json:"keys"`
	AWSIOTHost string `json:"aws_iot_host"`
}

var (
	errNoConfigFile = fmt.Errorf("config: file does not exists")
)

func loadConfig(path string) (*config, error) {
	expandedPath, err := homedir.Expand(path)
	if err != nil {
		return nil, err
	}

	fileBytes, err := ioutil.ReadFile(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errNoConfigFile
		}

		return nil, err
	}

	c := new(config)
	if err := json.Unmarshal(fileBytes, c); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *config) getTLSCertificate() (*tls.Certificate, error) {
	cer, err := tls.X509KeyPair([]byte(c.Certificates.Certificate), []byte(c.Certificates.PrivateKey))
	if err != nil {
		return nil, err
	}

	return &cer, nil
}
