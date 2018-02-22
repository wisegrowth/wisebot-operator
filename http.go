package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
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
	Version    string     `json:"version"`
}

type manageServiceHTTPRequest struct {
	Name string `json:"name"`
}

type mqttStatus struct {
	IsConnected bool `json:"is_connected"`
}

type updateHTTPPayload struct {
	NewVersion string `json:"version"`
}

func newHealthResponse() *healthResponse {
	data := new(healthzDataResponse)
	data.Services = processManager.Services
	data.Daemons = daemonStore

	meta := new(healthzMetaResponse)
	meta.MQTTStatus.IsConnected = processManager.MQTTClient.IsConnected()
	meta.Version = version

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

func restartServiceHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	payload := new(manageServiceHTTPRequest)
	if err := json.NewDecoder(r.Body).Decode(payload); err != nil {
		getLogger(r).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := processManager.Services.RestartService(payload.Name); err != nil {
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

func updateHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")
	payload := new(updateHTTPPayload)
	if err := json.NewDecoder(r.Body).Decode(payload); err != nil {
		getLogger(r).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	updater := NewUpdate(baseURL, version)
	err := updater.Update(payload.NewVersion)
	if err != nil {
		getLogger(r).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	processManager.Stop()
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		getLogger(r).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//TODO: implement support for daemon without a code's repository
	exec.Command("sudo", "systemctl", "restart", "operator").Run()

	return
}

func restartHTTPHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	processManager.Stop()

	//TODO: implement support for daemon without a code's repository
	exec.Command("sudo", "systemctl", "restart", "operator").Run()
	return
}

// NewHTTPServer returns an initialized server with the following routes:
//
// GET /healthz
// POST /service-start
// POST /service-stop
// POST /service-restart
// POST /update
// POST /restart
//
func NewHTTPServer() *http.Server {
	router := httprouter.New()

	router.GET("/healthz", healthzHTTPHandler)
	router.POST("/service-start", startServiceHTTPHandler)
	router.POST("/service-stop", stopServiceHTTPHandler)
	router.POST("/service-restart", restartServiceHTTPHandler)
	router.POST("/update", updateHTTPHandler)
	router.POST("/restart", restartHTTPHandler)

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
