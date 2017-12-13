package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/WiseGrowth/go-wisebot/logger"
	"github.com/WiseGrowth/go-wisebot/rasp"
	"github.com/WiseGrowth/wisebot-operator/daemon"
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
	MQTTStatus mqttStatus `json:"mqtt_status"`
}

type manageServiceHTTPRequest struct {
	Name string `json:"name"`
}

type mqttStatus struct {
	IsConnected bool `json:"is_connected"`
}

func newHealthResponse() *healthResponse {
	data := new(healthzDataResponse)
	data.Services = processManager.Services
	data.Daemons = daemonStore

	meta := new(healthzMetaResponse)
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

func startServiceHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	payload := new(manageServiceHTTPRequest)
	if err := json.NewDecoder(r.Body).Decode(payload); err != nil {
		getLogger(r).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := processManager.Services.StartService(payload.Name); err != nil {
		getLogger(r).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func stopServiceHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	payload := new(manageServiceHTTPRequest)
	if err := json.NewDecoder(r.Body).Decode(payload); err != nil {
		getLogger(r).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := processManager.Services.StopService(payload.Name); err != nil {
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
	router.POST("/service-start", startServiceHTTPHandler)
	router.POST("/service-stop", stopServiceHTTPHandler)

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
