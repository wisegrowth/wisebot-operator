package main

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
)

type config struct {
	WisebotID    string `json:"id"`
	Certificates struct {
		PrivateKey  []byte `json:"privateKey"`
		Certificate []byte `json:"certificate"`
	} `json:"keys"`
}

func loadConfig(path string) (*config, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	c := new(config)
	if err := json.Unmarshal(bytes, c); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *config) getTLSCertificate() (*tls.Certificate, error) {
	cer, err := tls.X509KeyPair(c.Certificates.Certificate, c.Certificates.PrivateKey)
	if err != nil {
		return nil, err
	}

	return &cer, nil
}
