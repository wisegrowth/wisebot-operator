package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/WiseGrowth/wisebot-operator/daemon"
	"github.com/WiseGrowth/wisebot-operator/led"
	"github.com/WiseGrowth/wisebot-operator/logger"
	"github.com/WiseGrowth/wisebot-operator/rasp"
	"github.com/julienschmidt/httprouter"
	"github.com/urfave/negroni"
)

type key int

const (
	httpPort = 5000

	loggerKey key = iota
)

type healthResponse struct {
	Data *healthzDataResponse `json:"data"`
	Meta *healthzMetaResponse `json:"meta"`
}

type healthzDataResponse struct {
	Services *ServiceStore `json:"services"`
	Daemons  *daemon.Store `json:"daemons"`
}

type healthzMetaResponse struct {
	WifiStatus wifiStatus `json:"wifi_status"`
	MQTTStatus mqttStatus `json:"mqtt_status"`
}

type wifiStatus struct {
	IsConnected bool   `json:"is_connected"`
	ESSID       string `json:"essid"`
}

type mqttStatus struct {
	IsConnected bool `json:"is_connected"`
}

func newHealthResponse() *healthResponse {
	isConnected, _ := rasp.IsConnected()
	currentESSID, _ := rasp.CurrentConfiguredNetworkESSID()

	data := new(healthzDataResponse)
	data.Services = processManager.Services
	data.Daemons = daemonStore

	meta := new(healthzMetaResponse)
	meta.WifiStatus.IsConnected = isConnected
	meta.WifiStatus.ESSID = currentESSID
	meta.MQTTStatus.IsConnected = processManager.MQTTClient.IsConnected()

	return &healthResponse{
		Data: data,
		Meta: meta,
	}
}

func getLogger(r *http.Request) logger.Logger {
	return r.Context().Value(loggerKey).(logger.Logger)
}

func healthzHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")
	payload := newHealthResponse()
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		getLogger(r).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func getNetworksHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")
	networks, err := rasp.AvailableNetworks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	payload := struct {
		Data []*rasp.Network `json:"data"`
	}{Data: networks}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		getLogger(r).Error(err)
	}
}

func updateNetworkHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	network := new(rasp.Network)
	if err := json.NewDecoder(r.Body).Decode(network); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// TODO: check if the network exists before configuring wifi

	log := getLogger(r)
	log.Debug(fmt.Sprintf("ESSID: %q Password: %q", network.ESSID, network.Password))

	go notifyInternetWithRetry(led.NetworkConnecting)

	err := rasp.SetupWifi(network)
	if err == nil {
		log.Debug("Wifi Connected")

		go notifyInternetWithRetry(led.NetworkConnected)

		const updateOnBootstrap = true
		processManager.KickOff(updateOnBootstrap)
		daemonStore.Bootstrap(updateOnBootstrap)
		w.WriteHeader(http.StatusOK)
		return
	}

	go notifyInternetWithRetry(led.NetworkError)

	if err == rasp.ErrNoWifi {
		log.Debug("No Wifi")
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	log.Error("Unexpected error: " + err.Error())
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// NewHTTPServer returns an initialized server with the following routes:
//
// GET /healthz
//
// GET    /networks
// PATCH  /network
//
func NewHTTPServer() *http.Server {
	router := httprouter.New()

	router.GET("/healthz", healthzHTTPHandler)

	router.GET("/networks", getNetworksHTTPHandler)

	router.PATCH("/network", updateNetworkHTTPHandler)

	addr := fmt.Sprintf(":%d", httpPort)
	routes := negroni.Wrap(router)
	n := negroni.New(negroni.HandlerFunc(httpLogginMiddleware), routes)
	server := &http.Server{Addr: addr, Handler: n}

	return server
}

func httpLogginMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	log := logger.GetLogger().WithField("route", r.URL.Path)
	now := time.Now()

	ctx := context.WithValue(r.Context(), loggerKey, log)
	r = r.WithContext(ctx)

	log.Debug("Request received")
	next(w, r)
	log.WithField("elapsed", time.Since(now).String()).Debug("Request end")
}

func notifyInternetWithRetry(status led.NetworkStatus) {
	now := time.Now()
	log := logger.GetLogger()
	for {
		if err := led.PostNetworkStatus(status, now); err != nil {
			log.Error(err)
			time.Sleep(3 * time.Second)
			log.Debug("network connected post failed, retrying")
			continue
		}

		log.Debug("network connected posted!")
		break
	}
}
