package models

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"time"
)

type (
	Confirmation struct {
		Id        string    `json:"id" bson:"_id,omitempty"`
		Key       string    `json:"key" bson:"key"`
		Type      Type      `json:"type" bson:"type"`
		Status    Status    `json:"status" bson:"status"`
		ToUser    string    `json:"createdBy" bson:"createdBy"`
		CreatorId string    `json:"creatorId" bson:"creatorId"` // could be empty
		Created   time.Time `json:"created" bson:"created"`     //used for expiry
		Modified  time.Time `json:"modified" bson:"modified"`   //sent - or maybe failed
	}

	//Enum type's
	Type   string
	Status string
)

const (
	PW_RESET        Type = "password_reset"
	CARETEAM_INVITE Type = "careteam_invitation"
	CONFIRMATION    Type = "email_confirmation"

	PENDING   Status = "pending"
	COMPLETED Status = "completed"
	DECLINED  Status = "declined"
)

func NewConfirmation(theType Type, to, from string) (*Confirmation, error) {

	if key, err := generateKey(); err != nil {
		return nil, err
	} else {

		conf := &Confirmation{
			Key:       key,
			Type:      theType,
			ToUser:    to,
			Status:    PENDING,
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
