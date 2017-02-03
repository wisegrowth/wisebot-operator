package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"text/template"
	"time"
)

func main() {
	// h := signatureKey(awsSecret, "us-west-2", "iotdata")
	// fmt.Println(hex.EncodeToString(h))

	// ai := &AWSInfo{
	// 	AccessKeyID: "accessID",
	// 	Region:      "us-west-2",
	// 	Service:     "iotdata",
	// }

	// ai.PrepareWebSocketURL()
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

// AWSInfo holds the needed data in order to connect
// to amazon services.
type AWSInfo struct {
	AccessKeyID string
	SecretKey   string
	Region      string
	Service     string
	Host        string
	Port        uint
}

// Hostname concatenates the host in lowercase and the port.
func (ai *AWSInfo) Hostname() string {
	return fmt.Sprintf("%s:%d", strings.ToLower(ai.Host), ai.Port)
}

// NewAWSInfo returns an initialized AWSInfo
func NewAWSInfo(accessID, region, service, host string, port uint, t time.Time) *AWSInfo {
	return &AWSInfo{
		AccessKeyID: accessID,
		Region:      region,
		Service:     service,
		Host:        host,
		Port:        port,
	}
}

const (
	queryParamTempl = `X-Amz-Algorithm=AWS4-HMAC-SHA256
	&X-Amz-Credential={{ .AccessKeyID }}%2F{{ .DateTime }}%2F{{ .Region }}%2F{{ .Service }}%2Faws4_request
	&X-Amz-Date={{ .Date }}
	&X-Amz-SignedHeaders=host
	`
)

var (
	queryParamTemplate = template.Must(template.New("queryParam").Parse(queryParamTempl))
)

type requestInfo struct {
	*AWSInfo

	Date     string
	DateTime string
}

type requestOptions struct {
	ClientID          string
	Debug             bool
	Protocol          string
	Port              uint
	WebSocketProtocol string
}

// PrepareWebSocketURL ...
func (ai *AWSInfo) PrepareWebSocketURL(options *requestOptions) string {
	now := time.Now().UTC()

	awsInfo := &requestInfo{
		AWSInfo:  ai,
		Date:     dateString(now),
		DateTime: dateTimeString(now),
	}

	var buf []byte
	bufw := bytes.NewBuffer(buf)
	queryParamTemplate.Execute(bufw, awsInfo)

	replacer := strings.NewReplacer(" ", "", "\n", "", "\t", "")
	queryParams := replacer.Replace(string(bufw.Bytes()))

	path := "/mqtt"
	hasher := sha256.New()
	hasher.Write([]byte(""))
	payload := hex.EncodeToString(hasher.Sum(nil))

	canonicalHeaders := fmt.Sprintf("host:%s\n", ai.Hostname())
	canonicalRequest := fmt.Sprintf("GET\n%s\n%s\n%s\nhost\n%s", path, queryParams, canonicalHeaders, payload)

	canonicalRequestHasher := sha256.New()
	canonicalRequestHasher.Write([]byte(canonicalRequest))

	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s/%s/%s/aws4_request\n%s", dateTimeString(now), dateString(now), ai.Region, ai.Service, canonicalRequestHasher.Sum(nil))
	signingKey := signatureKey(ai.SecretKey, ai.Region, ai.Service, now)

	signature := hmacSHA256(signingKey, stringToSign)

	return fmt.Sprintf("%s://%s%s?%s&X-Amz-Signature=%s", options.Protocol, ai.Hostname(), path, queryParams, signature)
}

func signatureKey(secret, region, service string, date time.Time) []byte {
	h := hmacSHA256([]byte("AWS4"+awsSecret), dateString(date))
	h = hmacSHA256(h, region)
	h = hmacSHA256(h, service)
	return hmacSHA256(h, "aws4_request")
}

const (
	dateFormat = "20060102T150405Z"
)
