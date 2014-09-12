package models

import (
	"crypto/rand"
	"encoding/base64"
	"labix.org/v2/mgo/bson"
	"log"
	"time"
)

type (
	Confirmation struct {
		Id        bson.ObjectId `json:"id" bson:"_id,omitempty"`
		Key       string        `json:"key" bson:"key"`
		Type      Type          `json:"type" bson:"type"`
		Status    Status        `json:"status" bson:"status"`
		ToUser    string        `json:"createdBy" bson:"createdBy"`
		CreatorId string        `json:"creatorId" bson:"creatorId"` // could be empty
		Created   time.Time     `json:"created" bson:"created"`
		Modified  time.Time     `json:"modified" bson:"modified"`
	}

	//Enum type's
	Type   string
	Status string
)

const (
	TypePasswordReset  Type = "password_reset"
	TypeCareteamInvite Type = "careteam_invitation"
	TypeConfirmation   Type = "email_confirmation"

	StatusPending   Status = "pending"
	StatusCompleted Status = "completed"
	StatusDeclined  Status = "declined"
)

func NewConfirmation(theType Type, to, from string) (*Confirmation, error) {

	if key, err := generateKey(); err != nil {
		return nil, err
	} else {

		conf := &Confirmation{
			Id:        bson.NewObjectId(),
			Key:       key,
			Type:      theType,
			ToUser:    to,
			Status:    StatusPending,
			CreatorId: from,
			Created:   time.Now(),
		}

		return conf, nil
	}
}

func generateKey() (string, error) {

	length := 24 // change the length of the generated random string here

	rb := make([]byte, length)
	if _, err := rand.Read(rb); err != nil {
		log.Println(err)
		return "", err
	} else {
		return base64.URLEncoding.EncodeToString(rb), nil
	}
}
