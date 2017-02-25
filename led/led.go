package led

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	urlString = "http://localhost:5005"
)

var apiURI = mustParseURL(urlString)

// NetworkStatus represents the raspberry pi network status
type NetworkStatus int

// Network statuses
const (
	NetworkConnected NetworkStatus = iota
	NetworkConnecting
	NetworkError
)

func (ns NetworkStatus) String() string {
	switch ns {
	case Connected:
		return "connected"
	case Connecting:
		return "connecting"
	case Error:
		return "error"
	}

	return "unknown-status"
}

// PostNetworkStatus sends a request to the led service acknowledging the
// wifi status.
func PostNetworkStatus(status NetworkStatus, when time.Time) error {
	payload := struct {
		Status    string `json:"status"`
		Timestamp int64
	}{
		Status:    status.String(),
		Timestamp: timeToTimestamp(when),
	}

	bf := new(bytes.Buffer)
	if err := json.NewEncoder(bf).Encode(payload); err != nil {
		return err
	}

	urlStr := buildURL("/network/" + status.String())
	req, err := http.NewRequest("POST", urlStr, bf)
	if err != nil {
		return err
	}
	client := http.Client{}
	_, err = client.Do(req)

	return err
}

func timeToTimestamp(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}

func buildURL(path string) string {
	for path[0] == '/' && len(path) > 1 {
		path = path[1:]
	}

	return fmt.Sprintf("%s/%s", apiURI.String(), path)
}

func mustParseURL(uri string) *url.URL {
	u, err := url.Parse(uri)

	if err != nil {
		panic(err)
	}

	return u
}