package clients

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	endpoint = "https://email.us-east-1.amazonaws.com"
)

type (
	SesNotifier struct {
		Config *SesNotifierConfig
	}
	SesNotifierConfig struct {
		from      string `json:"fromAddress"`
		secretKey string `json:"secretKey"`
		accessKey string `json:"accessKey"`
	}
)

func NewSesNotifier(cfg *SesNotifierConfig) *SesNotifier {
	return &SesNotifier{
		Config: cfg,
	}
}

func (c *SesNotifier) Send(to []string, subject string, msg string) (string, error) {

	data := make(url.Values)
	data.Add("Action", "SendEmail")
	data.Add("Source", c.Config.from)
	data.Add("Destination.ToAddresses.member.1", strings.Join(to, ", "))
	data.Add("Message.Subject.Data", subject)
	data.Add("Message.Body.Text.Data", msg)
	data.Add("AWSAccessKeyId", c.Config.accessKey)

	return c.sesPost(data)
}

func (c *SesNotifier) authorizationHeader(date string) []string {
	h := hmac.New(sha256.New, []uint8(c.Config.secretKey))
	h.Write([]uint8(date))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	auth := fmt.Sprintf("AWS3-HTTPS AWSAccessKeyId=%s, Algorithm=HmacSHA256, Signature=%s", c.Config.accessKey, signature)
	return []string{auth}
}

func (c *SesNotifier) sesPost(data url.Values) (string, error) {
	body := strings.NewReader(data.Encode())
	req, err := http.NewRequest("POST", endpoint, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	now := time.Now().UTC()
	// date format: "Tue, 25 May 2010 21:20:27 +0000"
	date := now.Format("Mon, 02 Jan 2006 15:04:05 -0700")
	req.Header.Set("Date", date)

	h := hmac.New(sha256.New, []uint8(c.Config.secretKey))
	h.Write([]uint8(date))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	auth := fmt.Sprintf("AWS3-HTTPS AWSAccessKeyId=%s, Algorithm=HmacSHA256, Signature=%s", c.Config.accessKey, signature)
	req.Header.Set("X-Amzn-Authorization", auth)

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("http error: %s", err)
		return "", err
	}

	resultbody, _ := ioutil.ReadAll(r.Body)
	r.Body.Close()

	if r.StatusCode != 200 {
		log.Printf("error, status = %d", r.StatusCode)

		log.Printf("error response: %s", resultbody)
		return "", errors.New(fmt.Sprintf("error code %d. response: %s", r.StatusCode, resultbody))
	}

	return string(resultbody), nil
}
