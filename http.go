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
	MQTTStatus          mqttStatus   `json:"mqtt_status"`
	WifiStatus          wifiStatus   `json:"wifi_status"`
	SSHTunnelStatus     tunnelStatus `json:"ssh_tunnel_status"`
	StorageTunnelStatus tunnelStatus `json:"storage_tunnel_status"`
	Version             string       `json:"version"`
}

type manageServiceHTTPRequest struct {
	Name string `json:"name"`
}

type mqttStatus struct {
	IsConnected bool `json:"is_connected"`
}

/*
{
  "data": {
    "is_connected":true,
    "essid":"OpenWrt"
  }
}
*/
type networkOperatorResponse struct {
	Data wifiStatus `json:"data"`
}
type wifiStatus struct {
	IsConnected bool   `json:"is_connected"`
	ESSID       string `json:"essid"`
	Error       bool   `json:"error"`
}

/*
{
  "data":{
    "status":"connected",
    "tunnel":{
      "id":"8d296d36a3062cd5",
      "url":"tcp://tunnel.wisegrowth.app:44483",
      "proto":"tcp",
      "port":"44483"
    }
  },
  "meta":{
    "timestamp":"2018-09-04T08:54:28.969Z",
    "version":"1.2.1"
  }
}
*/
type tunnelResponse struct {
	Data tunnelData `json:"data"`
	Meta tunnelMeta `json:"meta"`
}

type tunnelData struct {
	Status string     `json:"status"`
	Tunnel tunnelInfo `json:"tunnel"`
}

type tunnelInfo struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Proto string `json:"proto"`
	Port  string `json:"port"`
}

type tunnelMeta struct {
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}

type tunnelStatus struct {
	Status string `json:"status"`
	Port   string `json:"port"`
	Error  bool   `json:"error"`
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

	wifi := new(networkOperatorResponse)
	//TODO: get url from wisebot config file
	err := getJSON("http://localhost:5020/healthz", wifi)
	if err != nil {
		logger.GetLogger().Error(err)

		meta.WifiStatus.IsConnected = false
		meta.WifiStatus.ESSID = ""
		meta.WifiStatus.Error = true
	} else {
		meta.WifiStatus.IsConnected = wifi.Data.IsConnected
		meta.WifiStatus.ESSID = wifi.Data.ESSID
		meta.WifiStatus.Error = false
	}

	sshTunnel := new(tunnelResponse)
	//TODO: get url from wisebot config file
	err = getJSON("http://localhost:5025/healthz", sshTunnel)
	if err != nil {
		logger.GetLogger().Warn(err)

		meta.SSHTunnelStatus.Status = ""
		meta.SSHTunnelStatus.Port = ""
		meta.SSHTunnelStatus.Error = true
	} else {
		meta.SSHTunnelStatus.Status = sshTunnel.Data.Status
		meta.SSHTunnelStatus.Port = sshTunnel.Data.Tunnel.Port
		meta.SSHTunnelStatus.Error = false
	}

	storageTunnel := new(tunnelResponse)
	//TODO: get url from wisebot config file
	err = getJSON("http://localhost:5035/healthz", storageTunnel)
	if err != nil {
		logger.GetLogger().Warn(err)

		meta.StorageTunnelStatus.Status = ""
		meta.StorageTunnelStatus.Port = ""
		meta.StorageTunnelStatus.Error = true
	} else {
		meta.StorageTunnelStatus.Status = storageTunnel.Data.Status
		meta.StorageTunnelStatus.Port = storageTunnel.Data.Tunnel.Port
		meta.StorageTunnelStatus.Error = false
	}

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
