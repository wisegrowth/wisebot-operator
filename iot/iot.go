package iot

import (
	"crypto/tls"
	"fmt"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// Client ...
type Client struct {
	cert string
	key  string
	id   string

	host string
	port uint

	qos byte

	clientOptions *MQTT.ClientOptions
	MQTT.Client
}

// Connect creates a new mqtt client and uses the ClientOptions
// generated in the NewClient function to connect with
// the provided host and port.
func (c *Client) Connect() error {
	if c.Client != nil {
		return nil
	}

	mqttClient := MQTT.NewClient(c.clientOptions)
	if token := c.Client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	c.Client = mqttClient

	return nil
}

// Subscribe is a convenience function that proxies
// the function call to MQTT.Client.Subscribe in order
// to subscribe to  an specific topic and MQTT.MessageHandler.
func (c *Client) Subscribe(topic string, onMessage MQTT.MessageHandler) error {
	if token := c.Client.Subscribe(topic, c.qos, onMessage); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// Config ...
type Config func(*Client)

// NewClient ...
func NewClient(configs ...Config) *Client {
	client := &Client{
		port: 8883,
		qos:  byte(1),
	}

	for _, config := range configs {
		config(client)
	}

	cer, err := tls.LoadX509KeyPair(client.cert, client.key)
	if err != nil {
		panic(err)
	}

	client.clientOptions = &MQTT.ClientOptions{
		ClientID:             client.id,
		CleanSession:         true,
		MaxReconnectInterval: 1 * time.Second,
		KeepAlive:            30 * time.Second,
		TLSConfig:            tls.Config{Certificates: []tls.Certificate{cer}},
	}

	client.clientOptions.AddBroker(fmt.Sprintf("tcps://%s:%d/mqtt", client.host, client.port))

	return client
}

// SetCert ...
func SetCert(cert string) Config {
	return func(c *Client) {
		c.cert = cert
	}
}

// SetKey ...
func SetKey(key string) Config {
	return func(c *Client) {
		c.key = key
	}
}

// SetClientID ...
func SetClientID(id string) Config {
	return func(c *Client) {
		c.id = id
	}
}

// SetHost ...
func SetHost(host string) Config {
	return func(c *Client) {
		c.host = host
	}
}

// SetPort ...
func SetPort(port uint) Config {
	return func(c *Client) {
		c.port = port
	}
}

// SetQOS ...
func SetQOS(qos int) Config {
	return func(c *Client) {
		c.qos = byte(qos)
	}
}
