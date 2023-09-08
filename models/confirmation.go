package models

import (
	"bytes"
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
		ClinicId  string          `json:"clinicId,omitempty" bson:"clinicId,omitempty"`
		CreatorId string          `json:"creatorId" bson:"creatorId"`
		Creator   Creator         `json:"creator" bson:"creator"`
		Context   json.RawMessage `json:"context" bson:"context,omitempty"`
		Created   time.Time       `json:"created" bson:"created"`
		Modified  time.Time       `json:"modified" bson:"modified"`
		Status    Status          `json:"status" bson:"status"`

		Restrictions *Restrictions `json:"restrictions" bson:"-"`
		TemplateName TemplateName  `json:"-" bson:"templateName"`
		UserId       string        `json:"-" bson:"userId"`
	}

	//basic details for the creator of the confirmation
	Creator struct {
		*Profile   `json:"profile" bson:"-"`
		UserId     string `json:"userid" bson:"-"` //for compatability with blip
		ClinicId   string `json:"clinicId,omitempty" bson:"clinicId,omitempty"`
		ClinicName string `json:"clinicName,omitempty" bson:"clinicName,omitempty"`
	}
	Patient struct {
		Birthday      string `json:"birthday"`
		DiagnosisDate string `json:"diagnosisDate"`
		IsOtherPerson bool   `json:"isOtherPerson"`
		FullName      string `json:"fullName"`
	}
	Profile struct {
		FullName string  `json:"fullName"`
		Patient  Patient `json:"patient"`
	}

	Restrictions struct {
		CanAccept   bool   `json:"canAccept"`
		RequiredIdp string `json:"requiredIdp,omitempty"`
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
	TypePasswordReset   Type = "password_reset"
	TypeCareteamInvite  Type = "careteam_invitation"
	TypeClinicianInvite Type = "clinician_invitation"
	TypeSignUp          Type = "signup_confirmation"
	TypeNoAccount       Type = "no_account"
)

var (
	Timeouts TypeDurations = TypeDurations{
		TypeCareteamInvite: 7 * 24 * time.Hour,
		TypePasswordReset:  7 * 24 * time.Hour,
		TypeSignUp:         31 * 24 * time.Hour,
	}
)

// New confirmation with just the basics
func NewConfirmation(theType Type, templateName TemplateName, creatorId string) (*Confirmation, error) {

	if key, err := generateKey(); err != nil {
		return nil, err
	} else {

		conf := &Confirmation{
			Key:          key,
			Type:         theType,
			TemplateName: templateName,
			CreatorId:    creatorId,
			Creator:      Creator{}, //set before sending back to client
			Status:       StatusPending,
			Created:      time.Now(),
		}

		return conf, nil
	}
}

// New confirmation that includes context data
func NewConfirmationWithContext(theType Type, templateName TemplateName, creatorId string, data interface{}) (*Confirmation, error) {
	if conf, err := NewConfirmation(theType, templateName, creatorId); err != nil {
		return nil, err
	} else {
		conf.AddContext(data)
		return conf, nil
	}
}

// Add context data
func (c *Confirmation) AddContext(data interface{}) {

	jsonData, _ := json.Marshal(data)
	c.Context = jsonData
}

// Decode the context data into the provided type
func (c *Confirmation) DecodeContext(data interface{}) error {

	if c.Context != nil {
		if err := json.Unmarshal(c.Context, &data); err != nil {
			log.Printf("Err: %v\n", err)
			return err
		}
	}
	return nil
}

// Set a new status and update the modified time
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

func (c *Confirmation) ValidateClinicID(expectedClinicID string, validationErrors *[]error) *Confirmation {
	if expectedClinicID != c.ClinicId {
		*validationErrors = append(
			*validationErrors,
			fmt.Errorf("confirmation expected ClinicId of `%s` but had `%s`", expectedClinicID, c.UserId),
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

func (c *Confirmation) ValidateStatusIn(expectedStatuses []Status, validationErrors *[]error) *Confirmation {
	isValid := false
	for _, status := range expectedStatuses {
		if status == c.Status {
			isValid = true
			break
		}
	}

	if !isValid {
		*validationErrors = append(
			*validationErrors,
			fmt.Errorf("Confirmation expected Status in `%v` but had `%s`", expectedStatuses, c.Status),
		)
	}
	return c
}

func (c *Confirmation) ValidateType(expectedType Type, validationErrors *[]error) *Confirmation {
	if expectedType != c.Type {
		*validationErrors = append(
			*validationErrors,
			fmt.Errorf("Confirmation expected Type `%s` but had `%s`", expectedType, c.Type),
		)
	}
	return c
}

func (c *Confirmation) IsExpired() bool {
	timeout, ok := Timeouts[c.Type]
	if !ok {
		return false
	}

	return time.Now().After(c.Created.Add(timeout))
}

func (c *Confirmation) ResetKey() error {
	key, err := generateKey()
	if err != nil {
		return err
	}

	c.Key = key
	c.Status = StatusPending
	c.ResetCreationAttributes()

	return nil
}

func (c *Confirmation) ResetCreationAttributes() {
	c.Created = time.Now()
	c.Modified = time.Time{}
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

// AlertsConfig is included with a care team invitation to configure initial
// alerts for the invitation's recipient.
type AlertsConfig struct {
	UrgentLow       AlertConfigWithThreshold `json:"urgentLow"`
	Low             AlertConfigDeluxe        `json:"low"`
	High            AlertConfigDeluxe        `json:"high"`
	NotLooping      AlertConfigWithDelay     `json:"notLooping"`
	NoCommunication AlertConfigWithDelay     `json:"noCommunication"`
}

// AlertConfigBase describes the minimum specifics of a desired alert.
type AlertConfigBase struct {
	// Enabled controls whether notifications should be sent for this alert.
	Enabled bool
	// Repeat is measured in minutes.
	Repeat DurationMinutes `json:"repeat"`
}

// AlertConfigDelay mixes in a configurable delay to AlertConfigBase.
type AlertConfigDelay struct {
	// Delay is measured in minutes.
	Delay DurationMinutes `json:"delay,omitempty"`
}

// AlertConfigThreshold mixes in a configurable threshold to AlertConfigBase.
type AlertConfigThreshold struct {
	// Threshold is measured in mg/dL.
	Threshold int `json:"threshold"`
}

// AlertConfigWithThreshold extends AlertConfigBase with a configurable trigger
// threshold.
type AlertConfigWithThreshold struct {
	AlertConfigBase
	AlertConfigThreshold
}

// AlertConfigWithDelay extends AlertConfigBase with a configurable delay.
type AlertConfigWithDelay struct {
	AlertConfigBase
	AlertConfigDelay
}

// AlertConfigDeluxe extends AlertConfigBase with configurable delay and
// threshold.
type AlertConfigDeluxe struct {
	AlertConfigBase
	AlertConfigThreshold
	AlertConfigDelay
}

// DurationMinutes reads a JSON integer and converts it to a time.Duration.
type DurationMinutes time.Duration

func (m *DurationMinutes) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) || len(b) == 0 {
		*m = DurationMinutes(0)
		return nil
	}
	d, err := time.ParseDuration(string(b) + "m")
	if err != nil {
		return err
	}
	*m = DurationMinutes(d)
	return nil
}

func (m DurationMinutes) Duration() time.Duration {
	return time.Duration(m)
}
