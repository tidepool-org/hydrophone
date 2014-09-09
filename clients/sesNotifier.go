package clients

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type (
	SesNotifier struct {
		Config *SesNotifierConfig
	}
	SesNotifierConfig struct {
		EndPoint  string `json:"serverEndpoint"`
		From      string `json:"fromAddress"`
		SecretKey string `json:"secretKey"`
		AccessKey string `json:"accessKey"`
	}
)

func NewSesNotifier(cfg *SesNotifierConfig) *SesNotifier {
	return &SesNotifier{
		Config: cfg,
	}
}

func (c *SesNotifier) Send(to []string, subject string, msg string) (int, string) {

	data := make(url.Values)
	data.Add("Action", "SendEmail")
	data.Add("Source", c.Config.From)
	data.Add("Destination.ToAddresses.member.1", strings.Join(to, ", "))
	data.Add("Message.Subject.Data", subject)
	data.Add("Message.Body.Text.Data", msg)
	data.Add("AWSAccessKeyId", c.Config.AccessKey)

	return c.sesPost(data)
}

func (c *SesNotifier) generateAuthHeader(date string) string {
	h := hmac.New(sha256.New, []uint8(c.Config.SecretKey))
	h.Write([]uint8(date))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("AWS3-HTTPS AWSAccessKeyId=%s, Algorithm=HmacSHA256, Signature=%s", c.Config.AccessKey, signature)
}

func (c *SesNotifier) sesPost(data url.Values) (int, string) {
	body := strings.NewReader(data.Encode())
	req, err := http.NewRequest("POST", c.Config.EndPoint, body)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	date := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 -0700")
	req.Header.Set("Date", date)
	req.Header.Set("X-Amzn-Authorization", c.generateAuthHeader(date))

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("http error: %s", err)
		return http.StatusInternalServerError, err.Error()
	}

	resultbody, _ := ioutil.ReadAll(r.Body)
	r.Body.Close()

	return r.StatusCode, string(resultbody)
}
