package api

import (
	"net/http"

	"github.com/mdblp/shoreline/token"
)

type AuthMock struct {
	ServerToken  string
	Unauthorized bool
	UserID       string
	IsServer     bool
}

func NewAuthMock(userid string) *AuthMock {
	return &AuthMock{
		Unauthorized: false,
		UserID:       userid,
		IsServer:     true,
	}
}

func (client *AuthMock) Authenticate(req *http.Request) *token.TokenData {
	if client.Unauthorized {
		return nil
	}
	sessionToken := ""

	if req.Header.Get("x-tidepool-session-token") != "" {
		sessionToken = req.Header.Get("x-tidepool-session-token")
	}
	if sessionToken == "" {
		return nil
	}
	if sessionToken == testing_token_hcp {
		return &token.TokenData{UserId: client.UserID, IsServer: false, Role: "hcp"}
	}
	if sessionToken == testing_token_caregiver {
		return &token.TokenData{UserId: client.UserID, IsServer: false, Role: "caregiver"}
	}
	if sessionToken == testing_token_hcp2 {
		return &token.TokenData{UserId: testing_token_hcp2, IsServer: false, Role: "hcp"}
	}
	if sessionToken == testing_token_uid1 {
		return &token.TokenData{UserId: testing_uid1, IsServer: false, Role: "patient"}
	}
	return &token.TokenData{UserId: client.UserID, IsServer: false, Role: "patient"}
}
