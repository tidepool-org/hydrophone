package hydrophone

import (
	"encoding/json"
	"fmt"
	"github.com/mdblp/hydrophone/models"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/go-common/errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
)

type (
	ClientInterface interface {
		GetPendingInvitations(userID string, authToken string) ([]models.Confirmation, error)
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
		return nil, fmt.Errorf("Unable to parse urlString[%s]", client.host)
	}
	return theURL, nil
}

func (client *Client) GetPendingInvitations(userID string, authToken string) ([]models.Confirmation, error) {
	host, err := client.getHost()
	if err != nil {
		return nil, errors.New("No known hydrophone hosts")
	}
	host.Path = path.Join(host.Path, "invite", userID)
	req, _ := http.NewRequest("GET", host.String(), nil)
	req.Header.Add("x-tidepool-session-token", authToken)

	res, err := client.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failure to get pending invites")
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
