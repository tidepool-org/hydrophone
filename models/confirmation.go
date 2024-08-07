package models

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tidepool-org/go-common/clients"
	"github.com/tidepool-org/platform/alerts"
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
		ExpiresAt *time.Time      `json:"expiresAt,omitempty" bson:"expiresAt,omitempty"`

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

	AcceptPatientInvite struct {
		MRN       string   `json:"mrn"`
		BirthDate string   `json:"birthDate"`
		FullName  string   `json:"fullName"`
		Tags      []string `json:"tags"`
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

		if timeout, ok := Timeouts[theType]; ok {
			expiresAt := conf.Created.Add(timeout)
			conf.ExpiresAt = &expiresAt
		}
		return conf, nil
	}
}

// New confirmation that includes context data
func NewConfirmationWithContext(theType Type, templateName TemplateName, creatorId string, data interface{}) (*Confirmation, error) {
	if conf, err := NewConfirmation(theType, templateName, creatorId); err != nil {
		return nil, err
	} else {
		if err := conf.AddContext(data); err != nil {
			return nil, err
		}
		return conf, nil
	}
}

// Add context data
func (c *Confirmation) AddContext(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	c.Context = jsonData
	return nil
}

// HasPermissions checks for permissions with the given name.
func (c *Confirmation) HasPermission(name string) (bool, error) {
	perms := clients.Permissions{}
	err := c.DecodeContext(&perms)
	if err != nil {
		return false, err
	}
	return perms[name] != nil, nil
}

// Decode the context data into the provided type
func (c *Confirmation) DecodeContext(data interface{}) error {

	if c.Context != nil {
		if err := json.Unmarshal(c.Context, &data); err != nil {
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
	if c.ExpiresAt == nil || c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(*c.ExpiresAt)
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
		return "", err
	} else {
		return base64.URLEncoding.EncodeToString(rb), nil
	}
}

// CareTeamContext specifies details associated with a Care Team Confirmation.
type CareTeamContext struct {
	// Permissions to be granted if the Confirmation is accepted.
	Permissions clients.Permissions `json:"permissions"`
	// AlertsConfig is the initial configuration of alerts for the invitee.
	AlertsConfig *alerts.Config `json:"alertsConfig,omitempty"`
	// Nickname is a user-friendly name for the recipient of the invitation.
	Nickname *string `json:"nickname,omitempty"`
}

// UnmarshalJSON handles different iterations of Care Team Context.
//
// Originally the context was a go-common clients.Permissions
// (map[string]map[string]interface{}), but with care partner alerting it
// became necessary to handle both the older Permissions only Contexts, but
// also newer Contexts in which Permissions are stored under a key. In
// addition a hybrid Context is supported, where if permissions aren't found
// under a key, it's assumed that every other keys is an individual
// client.Permission.
//
// WARNING: this works only if the newly added context fields don't share a
// name with a previously used permission-type. Right now that means "note",
// "upload", and "view".
//
// If the API is migrated so this custom unmarshaler isn't necessary, that
// would be a good thing.
func (c *CareTeamContext) UnmarshalJSON(b []byte) error {
	// noCustomUnmarshaler temporarily disables the custom JSON unmarshaler.
	type noCustomUnmarshaler struct {
		CareTeamContext
		UnmarshalJSON struct{} `json:"-"`
	}

	generic := &noCustomUnmarshaler{}
	if err := json.Unmarshal(b, &generic); err != nil {
		return fmt.Errorf("unmarshaling Confirmation Context: %w", err)
	}
	if generic.AlertsConfig != nil {
		c.AlertsConfig = generic.AlertsConfig
	}
	if generic.Nickname != nil && *generic.Nickname != "" {
		c.Nickname = generic.Nickname
	}
	if generic.Permissions != nil {
		c.Permissions = generic.Permissions
	} else {
		// As there's no permissions key, this must be an older context.
		if err := json.Unmarshal(b, &c.Permissions); err != nil {
			return fmt.Errorf("unmarshaling Permissions: %w", err)
		}
		// Alternatively, one could unmarshal into a map, and iterate over it,
		// copying fields that don't match these below.
		delete(c.Permissions, "alertsConfig")
		delete(c.Permissions, "nickname")
	}

	return nil
}

func (c *CareTeamContext) Validate() error {
	if c.AlertsConfig != nil && c.Permissions["follow"] == nil {
		return fmt.Errorf("no alerts config without follow permission")
	}
	return nil
}
