package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/WiseGrowth/wisebot-operator/logger"
	"github.com/WiseGrowth/wisebot-operator/rasp"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	"github.com/urfave/negroni"
)

type key int

const (
	httpPort = 5000

	loggerKey key = iota
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

func getLogger(r *http.Request) logger.Logger {
	return r.Context().Value(loggerKey).(logger.Logger)
}

func healthzHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	payload := newHealthResponse()
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		getLogger(r).Error(err)
		return
	}
}

func getNetworksHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

func activateAPModeHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := rasp.ActivateAPMode(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func deactivateAPModeHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := rasp.DeactivateAPMode(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
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
	err := rasp.SetupWifi(network)
	if err == nil {
		log.Debug("Wifi Connected")
		// TODO: bootstrap services if it was in ap mode!
		w.WriteHeader(http.StatusOK)
		return
	}

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

	cors := cors.New(cors.Options{
		AllowedHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		AllowedMethods:     []string{"GET", "POST", "PATCH", "DELETE"},
		AllowCredentials:   true,
		OptionsPassthrough: true,
	})

	addr := fmt.Sprintf(":%d", httpPort)
	routes := negroni.Wrap(router)
	n := negroni.New(cors, negroni.HandlerFunc(httpLogginMiddleware), routes)
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
