package models

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"time"
)

type (
	Confirmation struct {
		Key       string          `json:"key" bson:"_id"`
		Type      Type            `json:"type" bson:"type"`
		Email     string          `json:"email" bson:"email"`
		CreatorId string          `json:"creatorId" bson:"creatorId"`
		Creator   string          `json:"creator" bson:"-"`
		Context   json.RawMessage `json:"context" bson:"context,omitempty"`
		Created   time.Time       `json:"created" bson:"created"`
		//don't reveal the status and modified data from consumers
		UserId   string    `json:"-" bson:"userId"`
		Status   Status    `json:"-" bson:"status"`
		Modified time.Time `json:"-" bson:"modified"`
	}

	//Enum type's
	Status string
	Type   string
)

const (
	//Available Status's
	StatusPending   Status = "pending"
	StatusCompleted Status = "completed"
	StatusCanceled  Status = "canceled"
	StatusDeclined  Status = "declined"
	//Available Type's
	TypePasswordReset  Type = "password_reset"
	TypeCareteamInvite Type = "careteam_invitation"
	TypeSignUp         Type = "signup_confirmation"
	TypeNoAccount      Type = "no_account"
)

//New confirmation with just the basics
func NewConfirmation(theType Type, creatorId string) (*Confirmation, error) {

	if key, err := generateKey(); err != nil {
		return nil, err
	} else {

		conf := &Confirmation{
			Key:       key,
			Type:      theType,
			CreatorId: creatorId,
			Status:    StatusPending,
			Created:   time.Now(),
		}

		return conf, nil
	}
}

//New confirmation that includes context data
func NewConfirmationWithContext(theType Type, creatorId string, data interface{}) (*Confirmation, error) {

	if conf, err := NewConfirmation(theType, creatorId); err != nil {
		return nil, err
	} else {
		conf.AddContext(data)
		return conf, nil
	}
}

//Add context data
func (c *Confirmation) AddContext(data interface{}) {

	jsonData, _ := json.Marshal(data)
	c.Context = jsonData
	return
}

//Decode the context data into the provided type
func (c *Confirmation) DecodeContext(data interface{}) error {

	if c.Context != nil {
		if err := json.Unmarshal(c.Context, &data); err != nil {
			log.Printf("Err: %v\n", err)
			return err
		}
	}
	return nil
}

//Set a new status and update the modified time
func (c *Confirmation) UpdateStatus(newStatus Status) {
	c.Status = newStatus
	c.Modified = time.Now()
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
