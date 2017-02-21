package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/WiseGrowth/wisebot-operator/logger"
	"github.com/WiseGrowth/wisebot-operator/rasp"
	"github.com/julienschmidt/httprouter"
)

const (
	httpPort = 5000
)

type healthResponse struct {
	Data *ServiceStore        `json:"data"`
	Meta *healthzMetaResponse `json:"meta"`
}

type healthzMetaResponse struct {
	WifiStatus wifiStatus `json:"wifi_status"`
}

type wifiStatus struct {
	IsConnected    bool `json:"is_connected"`
	IsAPModeActive bool `json:"is_ap_mode_active"`
}

func newHealthResponse() *healthResponse {
	isConnected, _ := rasp.IsConnected()
	isAPModeActive, _ := rasp.IsAPModeActive()

	meta := new(healthzMetaResponse)
	meta.WifiStatus.IsConnected = isConnected
	meta.WifiStatus.IsAPModeActive = isAPModeActive

	return &healthResponse{
		Data: services,
		Meta: meta,
	}
}

func healthzHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.GetLogger().WithField("handler", "healthzHTTPHandler")
	log.Info("Request received")

	payload := newHealthResponse()
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.GetLogger().Error(err)
		return
	}
}

func getNetworksHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.GetLogger().WithField("handler", "getNetworksHTTPHandler")
	log.Info("Request received")

	networks, err := rasp.AvailableNetworks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	payload := struct {
		Data []*rasp.Network `json:"data"`
	}{Data: networks}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.GetLogger().Error(err)
	}
}

func activateAPModeHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.GetLogger().WithField("handler", "activateAPModeHTTPHandler")
	log.Info("Request received")

	if err := rasp.ActivateAPMode(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func deactivateAPModeHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.GetLogger().WithField("handler", "deactivateAPModeHTTPHandler")
	log.Info("Request received")

	if err := rasp.DeactivateAPMode(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func updateNetworkHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.GetLogger().WithField("handler", "updateNetworkHTTPHandler")
	log.Info("Request received")

	network := new(rasp.Network)
	if err := json.NewDecoder(r.Body).Decode(network); err != nil {
		log.Debug("bad payload")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// TODO: check if the network exists before configuring wifi

	log.Debug(fmt.Sprintf("ESSID: %q Password: %q", network.ESSID, network.Password))
	err := rasp.SetupWifi(network)
	if err == nil {
		log.Debug("Wifi Connected")
		rasp.DeactivateAPMode()
		// TODO: bootstrap services if it was in ap mode!
		w.WriteHeader(http.StatusOK)
		return
	}

	if err == rasp.ErrNoWifi {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		if err = rasp.ActivateAPMode(); err != nil {
			log.Error("Error when setting AP Mode: " + err.Error())
		}
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
// POST   /network/ap-mode/activate
// POST   /network/ap-mode/deactivate
// PATCH  /network
//
func NewHTTPServer() *http.Server {
	router := httprouter.New()

	router.GET("/healthz", healthzHTTPHandler)

	router.GET("/networks", getNetworksHTTPHandler)

	router.POST("/network/ap-mode/activate", activateAPModeHTTPHandler)
	router.POST("/network/ap-mode/deactivate", deactivateAPModeHTTPHandler)

	router.PATCH("/network", updateNetworkHTTPHandler)

	addr := fmt.Sprintf(":%d", httpPort)
	server := &http.Server{Addr: addr, Handler: router}

	return server
}
