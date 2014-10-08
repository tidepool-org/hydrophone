package models

import (
	"encoding/json"
	"time"
)

type (
	//base 'invite' confirmation
	invite struct {
		Key        string          `json:"key" bson:"_id"`
		Permissons json.RawMessage `json:"permissons" bson:"context,omitempty"`
		Created    time.Time       `json:"created" bson:"created"`
	}
	//Invite a user has sent
	SentInvite struct {
		Email     string `json:"email" bson:"email"`
		CreatorId string `json:"creatorId" bson:"creatorId"`
		invite
	}
	//Invite a user has recieved
	RecievedInvite struct {
		Email     string `json:"email" bson:"email"`
		UserId    string `json:"userId" bson:"userId"`
		CreatorId string `json:"creatorId" bson:"creatorId"`
		invite
	}
	//Enum type's
	Type string
)

const (
	//Available Type's
	TypePasswordReset  Type = "password_reset"
	TypeCareteamInvite Type = "careteam_invitation"
	TypeConfirmation   Type = "email_confirmation"
)
