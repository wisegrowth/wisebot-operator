package iot

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"strings"
	"time"
)

const (
	dateFormat      = "20060102T150405Z"
	queryParamTempl = `X-Amz-Algorithm=AWS4-HMAC-SHA256
	&X-Amz-Credential={{ .AccessKeyID }}%2F{{ .DateTime }}%2F{{ .Region }}%2F{{ .Service }}%2Faws4_request
	&X-Amz-Date={{ .Date }}
	&X-Amz-SignedHeaders=host
	`
)

var (
	queryParamTemplate = template.Must(template.New("queryParam").Parse(queryParamTempl))
	spaceReplacer      = strings.NewReplacer(" ", "", "\n", "", "\t", "")
)

// Client holds the needed data in order to connect
// to amazon's iot service.
type Client struct {
	AccessKeyID string
	SecretKey   string
	Region      string
	Service     string
	Host        string
	Port        uint
}

// hostname concatenates the host in lowercase and the port.
func (c *Client) hostname() string {
	return fmt.Sprintf("%s:%d", strings.ToLower(c.Host), c.Port)
}

// NewClient returns an initialized Client
func NewClient(accessID, region, service, host string, port uint, t time.Time) *Client {
	return &Client{
		AccessKeyID: accessID,
		Region:      region,
		Service:     service,
		Host:        host,
		Port:        port,
	}
}

// PrepareWebSocketURL ...
func (c *Client) PrepareWebSocketURL(options *Options) string {
	now := time.Now().UTC()

	ri := &requestInfo{
		Client:   c,
		Date:     dateString(now),
		DateTime: dateTimeString(now),
	}

	var buf []byte
	bufw := bytes.NewBuffer(buf)
	queryParamTemplate.Execute(bufw, ri)

	queryParams := spaceReplacer.Replace(string(bufw.Bytes()))

	path := "/mqtt"
	hasher := sha256.New()
	hasher.Write([]byte(""))
	payload := hex.EncodeToString(hasher.Sum(nil))

	hostname := c.hostname()
	canonicalHeaders := fmt.Sprintf("host:%s\n", hostname)
	canonicalRequest := fmt.Sprintf("GET\n%s\n%s\n%s\nhost\n%s", path, queryParams, canonicalHeaders, payload)

	canonicalRequestHasher := sha256.New()
	canonicalRequestHasher.Write([]byte(canonicalRequest))

	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s/%s/%s/aws4_request\n%s", dateTimeString(now), dateString(now), c.Region, c.Service, canonicalRequestHasher.Sum(nil))
	signingKey := signatureKey(c.SecretKey, c.Region, c.Service, now)

	signature := hmacSHA256(signingKey, stringToSign)

	return fmt.Sprintf("%s://%s%s?%s&X-Amz-Signature=%s", options.Protocol, hostname, path, queryParams, signature)
}

func signatureKey(secret, region, service string, date time.Time) []byte {
	h := hmacSHA256([]byte("AWS4"+secret), dateString(date))
	h = hmacSHA256(h, region)
	h = hmacSHA256(h, service)
	return hmacSHA256(h, "aws4_request")
}

type requestInfo struct {
	*Client

	Date     string
	DateTime string
}

// Options represents the aws iot service options in order
// to connect to it.
type Options struct {
	ClientID          string
	Debug             bool
	Protocol          string
	Port              uint
	WebSocketProtocol string
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func dateTimeString(t time.Time) string {
	return t.UTC().Format(dateFormat)
}

func dateString(t time.Time) string {
	date := t.UTC()
	return fmt.Sprintf("%04d%02d%02d", date.Year(), date.Month(), date.Day())
}
