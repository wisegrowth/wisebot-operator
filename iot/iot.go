package iot

/*
This package let us use the aws iot service by using
the mqtt protocol in an easier way that using the raw
protocol.
*/

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	stdlog "log"
	"os"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/WiseGrowth/wisebot-operator/logger"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

const (
	secureProtocol = "tcps"
)

// SetDebug sets MQTT.DEBUG loggin
func SetDebug(debug bool) {
	if debug {
		MQTT.DEBUG = stdlog.New(os.Stdout, "[MQTT-DEBUG] ", 0)
		MQTT.CRITICAL = stdlog.New(os.Stdout, "[MQTT-CRITICAL] ", 0)
		MQTT.WARN = stdlog.New(os.Stdout, "[MQTT-WARN] ", 0)
		MQTT.ERROR = stdlog.New(os.Stdout, "[MQTT-ERROR] ", 0)
	} else {
		MQTT.DEBUG = stdlog.New(ioutil.Discard, "", 0)
		MQTT.CRITICAL = stdlog.New(ioutil.Discard, "", 0)
		MQTT.WARN = stdlog.New(ioutil.Discard, "", 0)
		MQTT.ERROR = stdlog.New(ioutil.Discard, "", 0)
	}
}

// Client is a wrapper on top of `MQTT.Client` that
// makes connecting to aws iot service easier.
type Client struct {
	id          string
	certificate tls.Certificate

	host string
	port uint
	path string

	qos byte

	clientOptions *MQTT.ClientOptions

	subscriptions subscriptionsStore

	sync.RWMutex
	MQTT.Client
}

type subscriptionsStore map[string]MQTT.MessageHandler

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
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	c.logger().Info("MQTT Connected")

	c.Lock()
	c.Client = mqttClient
	c.Unlock()

	return nil
}

// Disconnect proxies the function call to the MQTT.Client, but first checks if
// the client is not nil.
func (c *Client) Disconnect(quiesce uint) {
	if c.Client != nil {
		c.Client.Disconnect(quiesce)
	}
}

// Subscribe is a convenience function that proxies
// the function call to MQTT.Client.Subscribe in order
// to subscribe to  an specific topic and MQTT.MessageHandler.
func (c *Client) Subscribe(topic string, onMessage MQTT.MessageHandler) error {
	c.RLock()
	defer c.RUnlock()

	if token := c.Client.Subscribe(topic, c.qos, onMessage); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	if _, ok := c.subscriptions[topic]; ok {
		return fmt.Errorf("the topic %q is already subscribed", topic)
	}

	c.subscriptions[topic] = onMessage

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
		port:          8883,
		qos:           byte(1),
		path:          "/mqtt",
		subscriptions: make(subscriptionsStore),
	}

	for _, config := range configs {
		config(client)
	}

	copts := MQTT.NewClientOptions()
	copts.SetClientID(client.id)
	copts.SetMaxReconnectInterval(1 * time.Second)
	copts.SetOnConnectHandler(client.onConnect())
	copts.SetConnectionLostHandler(func(c MQTT.Client, err error) {
		logger.GetLogger().Warn("[MQTT] disconnected, reason: " + err.Error())
	})
	copts.SetTLSConfig(&tls.Config{Certificates: []tls.Certificate{client.certificate}})

	client.clientOptions = copts

	client.clientOptions.AddBroker(client.brokerURL(secureProtocol))

	return client, nil
}

func (c *Client) brokerURL(protocol string) string {
	return fmt.Sprintf("%s://%s:%d%s", protocol, c.host, c.port, c.path)
}

func (c *Client) logger() *logrus.Entry {
	return logger.GetLogger().WithField("broker", c.brokerURL(secureProtocol))
}

func (c *Client) onConnect() MQTT.OnConnectHandler {
	return func(client MQTT.Client) {
		c.logger().Debug("Running MQTT.OnConnectHandler")
		for topic, handler := range c.subscriptions {
			c.Subscribe(topic, handler)
		}
	}
}

// SetCertificate sets the client tls certificate.
func SetCertificate(cert tls.Certificate) Config {
	return func(c *Client) {
		c.certificate = cert
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

// SetQoS sets the client's QualityOfService level.
func SetQoS(qos int) Config {
	return func(c *Client) {
		c.qos = byte(qos)
	}
}
