package models

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type (
	Confirmation struct {
		Key       string          `json:"key" bson:"_id"`
		Type      Type            `json:"type" bson:"type"`
		Email     string          `json:"email" bson:"email"`
		CreatorId string          `json:"creatorId" bson:"creatorId"`
		Creator   Creator         `json:"creator" bson:"creator"`
		Context   json.RawMessage `json:"context" bson:"context,omitempty" swaggertype:"string" format:"base64"`
		Created   time.Time       `json:"created" bson:"created"`

		TemplateName TemplateName `json:"-" bson:"templateName"`
		UserId       string       `json:"-" bson:"userId"`
		Team         *Team        `json:"target" bson:",inline"`
		Role         string       `json:"role" bson:"role"`
		Status       Status       `json:"status" bson:"status"`
		Modified     time.Time    `json:"-" bson:"modified"`
		ShortKey     string       `json:"shortKey" bson:"shortKey"`
	}

	Team struct {
		ID   string `json:"id" bson:"teamId"`
		Name string `json:"name" bson:"-"`
	}
	//basic details for the creator of the confirmation
	Creator struct {
		*Profile `json:"profile" bson:"-"`
		UserId   string `json:"userid" bson:"-"` //for compatability with blip
	}
	Patient struct {
		Birthday      string `json:"birthday"`
		DiagnosisDate string `json:"diagnosisDate"`
		IsOtherPerson bool   `json:"isOtherPerson"`
		FullName      string `json:"fullName"`
	}
	Preferences struct {
		DisplayLanguage string `json:"displayLanguageCode"`
	}
	Profile struct {
		FullName string  `json:"fullName"`
		Patient  Patient `json:"patient"`
	}

	//Enum type's
	Status string
	Type   string

	Acceptance struct {
		Password string `json:"password"`
		Birthday string `json:"birthday"`
	}

	TypeDurations map[Type]time.Duration
)

const (
	//Available Status's
	StatusPending   Status = "pending"
	StatusCompleted Status = "completed"
	StatusCanceled  Status = "canceled"
	StatusDeclined  Status = "declined"
	//Available Type's
	TypePasswordReset            Type = "password_reset"
	TypePatientPasswordReset     Type = "patient_password_reset"
	TypePatientPasswordInfo      Type = "patient_password_info"
	TypeCareteamInvite           Type = "careteam_invitation"    // invite and share data to a caregiver
	TypeMedicalTeamInvite        Type = "medicalteam_invitation" // invite an hcp to a medical team
	TypeMedicalTeamPatientInvite Type = "medicalteam_patient_invitation"
	TypeMedicalTeamDoAdmin       Type = "medicalteam_do_admin"
	TypeMedicalTeamRemove        Type = "medicalteam_remove"
	TypeSignUp                   Type = "signup_confirmation"
	TypeNoAccount                Type = "no_account"
	TypeInformation              Type = "patient_information"
	TypePatientPinReset          Type = "patient_pin_reset"
	shortKeyLength                    = 8
	letterBytes                       = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

var (
	Timeouts TypeDurations = TypeDurations{
		TypeCareteamInvite:       7 * 24 * time.Hour,
		TypePasswordReset:        7 * 24 * time.Hour,
		TypeSignUp:               31 * 24 * time.Hour,
		TypePatientPasswordReset: 1 * time.Hour,
		TypeMedicalTeamInvite:    7 * 24 * time.Hour,
	}
)

//New confirmation with just the basics
func NewConfirmation(theType Type, templateName TemplateName, creatorId string) (*Confirmation, error) {

	shortKey := ""
	status := StatusPending
	var err error = nil

	switch theType {
	case TypePatientPasswordReset:
		if shortKey, err = generateShortKey(shortKeyLength); err != nil {
			return nil, err
		}
	case TypePatientPasswordInfo:
		status = StatusCompleted
	default:
	}
	if key, err := generateKey(); err != nil {
		return nil, err
	} else {

		conf := &Confirmation{
			Key:          key,
			Type:         theType,
			TemplateName: templateName,
			CreatorId:    creatorId,
			Creator:      Creator{}, //set before sending back to client
			Team:         &Team{},
			Status:       status,
			Created:      time.Now(),
			ShortKey:     shortKey,
		}

		return conf, nil
	}
}

//New confirmation that includes context data
func NewConfirmationWithContext(theType Type, templateName TemplateName, creatorId string, data interface{}) (*Confirmation, error) {
	if conf, err := NewConfirmation(theType, templateName, creatorId); err != nil {
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

func (c *Confirmation) ValidateCreatorID(expectedCreatorID string, validationErrors *[]error) *Confirmation {
	if expectedCreatorID != c.CreatorId {
		*validationErrors = append(
			*validationErrors,
			fmt.Errorf("Confirmation expected CreatorID `%s` but had `%s`", expectedCreatorID, c.CreatorId),
		)
	}
	return c
}

func (c *Confirmation) ValidateUserID(expectedUserID string, validationErrors *[]error) *Confirmation {
	if expectedUserID != c.UserId {
		*validationErrors = append(
			*validationErrors,
			fmt.Errorf("Confirmation expected UserID of `%s` but had `%s`", expectedUserID, c.UserId),
		)
	}
	return c
}

func (c *Confirmation) ValidateTeamID(expectedTeamID string, validationErrors *[]error) *Confirmation {
	if c.Team != nil && expectedTeamID != c.Team.ID {
		*validationErrors = append(
			*validationErrors,
			fmt.Errorf("Confirmation expected TeamId of `%s` but had `%s`", expectedTeamID, c.Team.ID),
		)
	}
	return c
}

func (c *Confirmation) ValidateTeamName(expectedTeamName string, validationErrors *[]error) *Confirmation {
	if c.Team != nil && expectedTeamName != c.Team.Name {
		*validationErrors = append(
			*validationErrors,
			fmt.Errorf("Confirmation expected TeamName of `%s` but had `%s`", expectedTeamName, c.Team.Name),
		)
	}
	return c
}

func (c *Confirmation) ValidateStatus(expectedStatus Status, validationErrors *[]error) *Confirmation {
	if expectedStatus != c.Status {
		*validationErrors = append(
			*validationErrors,
			fmt.Errorf("Confirmation expected Status of `%s` but had `%s`", expectedStatus, c.Status),
		)
	}
	return c
}

func (c *Confirmation) ValidateType(expectedTypes []Type, validationErrors *[]error) *Confirmation {
	types := ""
	for _, expectedType := range expectedTypes {
		if expectedType == c.Type {
			return c
		}
		types = fmt.Sprintf("%s, %s", types, expectedType)
	}
	*validationErrors = append(
		*validationErrors,
		fmt.Errorf("Confirmation expected Type(s) `%s` but had `%s`", types, c.Type),
	)
	return c
}

func (c *Confirmation) IsExpired() bool {
	timeout, ok := Timeouts[c.Type]
	if !ok {
		log.Printf("[%s] does not exist", c.Type)
		return false
	}

	return time.Now().After(c.Created.Add(timeout))
}

func (c *Confirmation) ResetKey() error {
	key, err := generateKey()
	if err != nil {
		return err
	}
	shortKey, err := generateShortKey(shortKeyLength)
	if err != nil {
		return err
	}

	c.Key = key
	c.Status = StatusPending
	c.Created = time.Now()
	c.Modified = time.Time{}
	c.ShortKey = shortKey

	return nil
}

func GenerateRandomBytes(length int) ([]byte, error) {
	rb := make([]byte, length)
	if _, err := rand.Read(rb); err != nil {
		log.Println(err)
		return nil, err
	} else {
		return rb, nil
	}
}

func generateKey() (string, error) {
	length := 24 // change the length of the generated random string here
	rb, err := GenerateRandomBytes(length)
	return base64.URLEncoding.EncodeToString(rb), err
}

func generateShortKey(length int) (string, error) {
	bytes, err := GenerateRandomBytes(length)
	if err != nil {
		return "", err
	}
	for i, b := range bytes {
		bytes[i] = letterBytes[b%byte(len(letterBytes))]
	}
	return string(bytes), nil
}
