package infrastructure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/mdblp/go-common/v2/blperr"
	"github.com/mdblp/go-common/v2/clients/status"
	"github.com/mdblp/go-common/v2/http/request"

	"github.com/mdblp/hydrophone/models"
)

const serverAuthErrorKind = "auth-connection"

type Client struct {
	host       string       // host url
	httpClient *http.Client // store a reference to the http client so we can reuse it
}

func NewAuthClient(client *http.Client) *Client {
	host := MustGetConfigFromEnvironment("AUTH_URL")
	return &Client{
		host:       host,
		httpClient: client,
	}
}

// Get user details for the given user (from legacy auth system)
// In this case the userID could be the actual ID or an email address
func (client *Client) GetUser(userId, token string) (*models.UserData, error) {
	ctx := context.Background()
	req, err := request.NewGetBuilder(client.host).
		WithPath("user", userId).
		WithAuthToken(token).Build(ctx)

	if err != nil {
		return nil, blperr.Newf(serverAuthErrorKind, "Failure to get a user. Error = %v", err)
	}
	res, err := client.httpClient.Do(req)
	if err != nil {
		return nil, blperr.Newf(serverAuthErrorKind, "Failure to get a user. Error = %v", err)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		ud, err := extractUserData(res.Body)
		if err != nil {
			return nil, err
		}
		return ud, nil
	case http.StatusNoContent:
		return &models.UserData{}, nil
	default:
		return nil, &status.StatusError{
			Status: status.NewStatusf(res.StatusCode, "Unknown response code from service[%s]", req.URL),
		}
	}
}

// Update a user in backloops legacy authentication db (shoreline)
func (client *Client) UpdateUser(userId string, userUpdate models.UserUpdate, token string) error {
	ctx := context.Background()
	//structure that the update are given to us in
	type updatesToApply struct {
		Updates models.UserUpdate `json:"updates"`
	}

	req, err := request.NewPutBuilder(client.host).
		WithPath("user", userId).
		WithAuthToken(token).WithPayload(updatesToApply{Updates: userUpdate}).
		Build(ctx)

	if err != nil {
		return blperr.Newf(serverAuthErrorKind, "Failure to update a user. Error = %v", err)
	}

	res, err := client.httpClient.Do(req)
	if err != nil {
		return blperr.Newf(serverAuthErrorKind, "Failure to update a user. Error = %v", err)
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

func extractUserData(r io.Reader) (*models.UserData, error) {
	var ud models.UserData
	if err := json.NewDecoder(r).Decode(&ud); err != nil {
		return nil, err
	}
	return &ud, nil
}
