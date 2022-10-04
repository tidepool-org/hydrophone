package hydrophone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/mdblp/go-common/clients/status"
	"github.com/mdblp/go-common/errors"
	"github.com/mdblp/hydrophone/models"
)

type (
	ClientInterface interface {
		GetPendingInvitations(userID string, authToken string) ([]models.Confirmation, error)
		GetPendingSignup(userID string, authToken string) (*models.Confirmation, error)
		CancelSignup(confirm models.Confirmation, authToken string) error
		SendNotification(topic string, notif interface{}, authToken string) error
	}

	Client struct {
		host       string       // host url
		httpClient *http.Client // store a reference to the http client so we can reuse it
	}

	ClientBuilder struct {
		host       string       // host url
		httpClient *http.Client // store a reference to the http client so we can reuse it
	}
)

func NewHydrophoneClientBuilder() *ClientBuilder {
	return &ClientBuilder{}
}

// WithHost set the host
func (b *ClientBuilder) WithHost(host string) *ClientBuilder {
	b.host = host
	return b
}

// WithHTTPClient set the HTTP client
func (b *ClientBuilder) WithHTTPClient(httpClient *http.Client) *ClientBuilder {
	b.httpClient = httpClient
	return b
}

// Build return client from builder
func (b *ClientBuilder) Build() *Client {

	if b.host == "" {
		panic("Hydrophone client requires a host to be set")
	}
	if b.httpClient == nil {
		b.httpClient = http.DefaultClient
	}

	return &Client{
		httpClient: b.httpClient,
		host:       b.host,
	}
}

// NewHydrophoneClientFromEnv read the config from the environment variables
func NewHydrophoneClientFromEnv(httpClient *http.Client) *Client {
	builder := NewHydrophoneClientBuilder()
	host, _ := os.LookupEnv("HYDROPHONE_HOST")
	return builder.WithHost(host).
		WithHTTPClient(httpClient).
		Build()
}

func (client *Client) getHost() (*url.URL, error) {
	if client.host == "" {
		return nil, errors.New("No client host defined")
	}
	theURL, err := url.Parse(client.host)
	if err != nil {
		return nil, fmt.Errorf("unable to parse urlString[%s]", client.host)
	}
	return theURL, nil
}

func (client *Client) GetPendingInvitations(userID string, authToken string) ([]models.Confirmation, error) {
	return client.GetPendingInviteOrSignup(userID, authToken, models.TypeCareteamInvite)
}

func (client *Client) GetPendingSignup(userID string, authToken string) (*models.Confirmation, error) {
	res, err := client.GetPendingInviteOrSignup(userID, authToken, models.TypeSignUp)

	if err != nil {
		return nil, err
	} else if len(res) > 1 {
		return nil, fmt.Errorf("more than one signup found for %s", userID)
	} else if len(res) == 1 {
		return &res[0], err
	} else {
		return nil, nil
	}
}

func (client *Client) GetPendingInviteOrSignup(userID string, authToken string, confirmType models.Type) ([]models.Confirmation, error) {
	host, err := client.getHost()
	if err != nil {
		return nil, errors.New("No known hydrophone hosts")
	}

	if confirmType == models.TypeSignUp {
		host.Path = path.Join(host.Path, "signup", userID)
	} else {
		host.Path = path.Join(host.Path, "invite", userID)
	}
	req, _ := http.NewRequest("GET", host.String(), nil)
	req.Header.Add("x-tidepool-session-token", authToken)

	res, err := client.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failure to get pending confirm")
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		confirmations := make([]models.Confirmation, 0)
		if err = json.NewDecoder(res.Body).Decode(&confirmations); err != nil {
			log.Println("Error parsing JSON results", err)
			return nil, err
		}
		return confirmations, nil

	case http.StatusNotFound:
		return []models.Confirmation{}, nil
	default:
		return nil, &status.StatusError{
			Status: status.NewStatusf(res.StatusCode, "Unknown response code from service[%s]", req.URL),
		}
	}
}

func (client *Client) CancelSignup(confirm models.Confirmation, authToken string) error {
	host, err := client.getHost()
	if err != nil {
		return errors.New("No known hydrophone hosts")
	}

	host.Path = path.Join(host.Path, "signup", confirm.UserId)

	req, _ := http.NewRequest("PUT", host.String(), nil)
	req.Header.Add("x-tidepool-session-token", authToken)

	data, err := json.Marshal(confirm)
	if err != nil {
		return errors.Wrap(err, "Failure to marshal confirmation")
	}

	req.Body = ioutil.NopCloser(bytes.NewReader(data))

	res, err := client.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "Failure to cancel signup")
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		return nil
	default:
		return &status.StatusError{
			Status: status.NewStatusf(res.StatusCode, "Unknown response code from service[%s]", req.URL),
		}
	}
}

func (h *Client) SendNotification(topic string, notif interface{}, authToken string) error {
	host, err := h.getHost()
	if err != nil {
		return errors.New("No known hydrophone hosts")
	}

	host.Path = path.Join(host.Path, "notifications", topic)

	req, _ := http.NewRequest("POST", host.String(), nil)
	req.Header.Add("x-tidepool-session-token", authToken)

	data, err := json.Marshal(notif)
	if err != nil {
		return errors.Wrap(err, "Failure to marshal notification")
	}

	req.Body = ioutil.NopCloser(bytes.NewReader(data))

	res, err := h.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "Failure to send notification to hydrophone")
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		return nil
	default:
		return &status.StatusError{
			Status: status.NewStatusf(res.StatusCode, "Unknown response code from service[%s]", req.URL),
		}
	}
}
