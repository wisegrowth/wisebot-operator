package iot

/*
This package let us use the aws iot service by using
the mqtt protocol in an easier way that using the raw
protocol.
*/

import (
	"crypto/tls"
	"fmt"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// Client is a wrapper on top of `MQTT.Client` that
// makes connecting to aws iot service easier.
type Client struct {
	cert string
	key  string
	id   string

	host string
	port uint
	path string

	qos byte

	clientOptions *MQTT.ClientOptions
	MQTT.Client
}

// Connect creates a new mqtt client and uses the ClientOptions
// generated in the NewClient function to connect with
// the provided host and port.
// This method takes the client's host, port and path and generates
// the broker url where to connect.
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

// Config represents an attribute config setter for the
// `Client`.
type Config func(*Client)

// NewClient returns a configured `Client`. Is mandatory
// to provide valid tls certificates or it'll return an error
// instead.
// By default it generates a client with:
// - port: 8883
// - qos: 1
// - path: /mqtt
func NewClient(configs ...Config) (*Client, error) {
	client := &Client{
		port: 8883,
		qos:  byte(1),
		path: "/mqtt",
	}

	for _, config := range configs {
		config(client)
	}

	cer, err := tls.LoadX509KeyPair(client.cert, client.key)
	if err != nil {
		return nil, err
	}

	client.clientOptions = &MQTT.ClientOptions{
		ClientID:             client.id,
		CleanSession:         true,
		MaxReconnectInterval: 1 * time.Second,
		KeepAlive:            30 * time.Second,
		TLSConfig:            tls.Config{Certificates: []tls.Certificate{cer}},
	}

	client.clientOptions.AddBroker(fmt.Sprintf("tcps://%s:%d%s", client.host, client.port, client.path))

	return client, nil
}

// SetCert sets the client ssl certificate.
func SetCert(cert string) Config {
	return func(c *Client) {
		c.cert = cert
	}
}

// SetKey sets the client ssl private key.
func SetKey(key string) Config {
	return func(c *Client) {
		c.key = key
	}
}

// SetClientID sets the mqtt client id.
func SetClientID(id string) Config {
	return func(c *Client) {
		c.id = id
	}
}

// SetHost sets the host where to connect.
func SetHost(host string) Config {
	return func(c *Client) {
		c.host = host
	}
}

// SetPort sets the port where to connect.
func SetPort(port uint) Config {
	return func(c *Client) {
		c.port = port
	}
}

// SetPath sets the path where to connect.
func SetPath(path string) Config {
	return func(c *Client) {
		c.path = path
	}
}

// SetQOS sets the client's QualityOfService level.
func SetQOS(qos int) Config {
	return func(c *Client) {
		c.qos = byte(qos)
	}
}
