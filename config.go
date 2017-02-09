package main

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
)

type config struct {
	WisebotID    string `json:"id"`
	Certificates struct {
		PrivateKey  string `json:"privateKey"`
		Certificate string `json:"certificate"`
	} `json:"keys"`
}

func loadConfig(path string) (*config, error) {
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
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
