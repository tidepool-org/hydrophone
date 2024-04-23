package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

const (
	USERID  = "1234-555"
	USERID2 = "6789-000"
)

type Extras struct {
	Blah  string `json:"blah"`
	Email string `json:"email"`
}

var contextData = &Extras{Blah: "stuff", Email: "test@user.org"}

func Test_NewConfirmation(t *testing.T) {

	confirmation := MustConfirmation(t, TypePasswordReset, TemplateNamePasswordReset, USERID)

	if confirmation.Status != StatusPending {
		t.Fatalf("Status should be [%s] but is [%s]", StatusPending, confirmation.Status)
	}

	if confirmation.Key == "" {
		t.Fatal("There should be a generated key")
	}

	if confirmation.Created.IsZero() {
		t.Fatal("The created time should be set")
	}

	if confirmation.Modified.IsZero() == false {
		t.Fatal("The modified time should NOT be set")
	}

	if confirmation.Type != TypePasswordReset {
		t.Fatalf("The type should be [%s] but is [%s]", TypePasswordReset, confirmation.Type)
	}

	if confirmation.TemplateName != TemplateNamePasswordReset {
		t.Fatalf("The template type should be [%s] but is [%s]", TemplateNamePasswordReset, confirmation.TemplateName)
	}

	if confirmation.UserId != "" {
		t.Logf("expected '' actual [%s]", confirmation.UserId)
		t.Fail()
	}

	if confirmation.CreatorId != USERID {
		t.Logf("expected [%s] actual [%s]", USERID, confirmation.CreatorId)
		t.Fail()
	}

	if confirmation.Creator.Profile != nil {
		t.Logf("expected `nil` actual [%v]", confirmation.Creator.Profile)
		t.Fail()
	}

	if confirmation.Creator.UserId != "" {
		t.Logf("expected `` actual [%s]", confirmation.Creator.UserId)
		t.Fail()
	}

	if confirmation.ExpiresAt.IsZero() {
		t.Errorf("expected expiresAt to be non-Zero")
	}

	confirmation.UpdateStatus(StatusCompleted)

	if confirmation.Status != StatusCompleted {
		t.Fatalf("Status should be [%s] but is [%s]", StatusCompleted, confirmation.Status)
	}

	if confirmation.Modified.IsZero() != false {
		t.Fatal("The modified time should have been set")
	}

}

func Test_NewConfirmationWithContext(t *testing.T) {

	confirmation := MustConfirmationWithContext(t, TypePasswordReset, TemplateNamePasswordReset, USERID, contextData)

	myExtras := &Extras{}

	confirmation.DecodeContext(&myExtras)

	if myExtras.Blah == "" {
		t.Fatalf("context not decoded [%v]", myExtras)
	}

	if myExtras.Email == "" {
		t.Fatalf("context not decoded [%v]", myExtras)
	}

	//and all tests should pass for a new confirmation too
	Test_NewConfirmation(t)
}

func Test_Confirmation_AddContext(t *testing.T) {

	confirmation, err := NewConfirmation(TypePasswordReset, TemplateNamePasswordReset, USERID)
	if err != nil {
		t.Fatalf("expected nil, got %+v", err)
	}
	if err := confirmation.AddContext(contextData); err != nil {
		t.Fatalf("error adding context: %s", err)
	}

	myExtras := &Extras{}

	err = confirmation.DecodeContext(&myExtras)
	if err != nil {
		t.Fatalf("expected nil, got %+v", err)
	}

	if myExtras.Blah == "" {
		t.Fatalf("context not decoded [%v]", myExtras)
	}

	if myExtras.Email == "" {
		t.Fatalf("context not decoded [%v]", myExtras)
	}

}

func TestConfirmationKey(t *testing.T) {

	key, err := generateKey()
	if err != nil {
		t.Fatalf("error generating key: %s", err)
	}

	if key == "" {
		t.Fatal("There should be a generated key")
	}

	if len(key) != 32 {
		t.Fatal("The generated key should be 32 chars: ", len(key))
	}
}

func TestConfirmationContextCustomUnmarshaler(s *testing.T) {
	s.Run("handles original-recipe Context (aka bare Permissions)", func(t *testing.T) {
		oldContext := buff(`{"view":{}}`)
		cc := &CareTeamContext{}
		if err := json.NewDecoder(oldContext).Decode(cc); err != nil {
			t.Fatalf("expected nil, got %+v", err)
		}
		if view := cc.Permissions["view"]; view == nil {
			t.Fatal("expected view permissions to not be nil, got nil")
		}
	})

	s.Run("handles a hybrid Context with alerts.Config and old-style permissions", func(t *testing.T) {
		hybridContext := buff(`{
  "view": {},
  "alertsConfig": {
    "userId": "%s",
    "followedId": "%s",
    "low": {
      "enabled": true,
      "repeat": 30,
      "delay": 10,
      "threshold": {
        "units": "mg/dL",
        "value": 100
      }
    }
  }
}`, USERID, USERID2)

		cc := &CareTeamContext{}
		if err := json.NewDecoder(hybridContext).Decode(cc); err != nil {
			t.Fatalf("expected nil, got %+v", err)
		}
		if view := cc.Permissions["view"]; view == nil {
			t.Fatal("expected view permissions to not be nil, got nil")
		}
		if alerts := cc.AlertsConfig; alerts == nil {
			t.Fatal("expected alerts config to not be nil, got nil")
		}
		if low := cc.AlertsConfig.Low; low.Value != 100 {
			t.Fatalf("expected 100, got %f", low.Value)
		}
	})

	s.Run("handles a new-style Context with unexpected keys", func(t *testing.T) {
		newContext := buff(`{
  "permissions": {
    "view": {}
  },
  "alertsConfig": {
    "userId": "%s",
    "followedId": "%s",
    "low": {
      "enabled": true,
      "repeat": 30,
      "delay": 10,
      "threshold": {
        "units": "mg/dL",
        "value": 100
      }
    }
  },
  "ignored": {}
}`, USERID, USERID2)

		cc := &CareTeamContext{}
		if err := json.NewDecoder(newContext).Decode(cc); err != nil {
			t.Fatalf("expected nil, got %s", err)
		}
		if view := cc.Permissions["view"]; view == nil {
			t.Fatal("expected view permissions to not be nil, got nil")
		}
		// Since a "permissions" key is found, any additional keys (like
		// "ignored") should be… ignored.
		if cc.Permissions["ignored"] != nil {
			t.Fatal("expected \"ignored\" to be ignored, but is present")
		}
		if alerts := cc.AlertsConfig; alerts == nil {
			t.Fatal("expected alerts config to not be nil, got nil")
		}
		if low := cc.AlertsConfig.Low; low.Value != 100 {
			t.Fatalf("expected 100, got %f", low.Value)
		}
	})
}

func TestConfirmationCalculatesExpiresAt(t *testing.T) {
	for cType := range Timeouts {
		invite, err := NewConfirmation(cType, TemplateNamePasswordReset, USERID)
		if err != nil {
			t.Fatalf("expected nil, got %+v", err)
		}
		if invite.ExpiresAt == nil || invite.ExpiresAt.IsZero() {
			t.Errorf("expected non-Zero ExiresAt")
		}
	}
}

func TestConfirmationIsExpired(t *testing.T) {
	for cType := range Timeouts {
		invite, err := NewConfirmation(cType, TemplateNameCareteamInvite, USERID)
		if err != nil {
			t.Fatalf("expected nil error, got %s", err)
		}
		if invite.IsExpired() {
			t.Errorf("expected false, got true")
		}
		*invite.ExpiresAt = time.Unix(0, 0)
		if !invite.IsExpired() {
			t.Errorf("expected true, got false")
		}
	}

	// These types don't have timeouts, so don't get an expires at. They're
	// never expired.
	for _, cType := range []Type{TypeClinicianInvite, TypeNoAccount} {
		nonExpiringInviting, err := NewConfirmation(cType, TemplateNameCareteamInvite, USERID)
		if err != nil {
			t.Fatalf("expected nil error, got %s", err)
		}
		if nonExpiringInviting.IsExpired() {
			t.Errorf("expected invitation type %q to never expire", cType)
		}
	}

}

// buff is a helper for generating a JSON []byte representation.
func buff(format string, args ...interface{}) *bytes.Buffer {
	return bytes.NewBufferString(fmt.Sprintf(format, args...))
}

// MustConfirmation is a helper for tests that fails the test when
// confirmation creation fails.
func MustConfirmation(t *testing.T, theType Type, templateName TemplateName,
	creatorID string) *Confirmation {

	c, err := NewConfirmation(theType, templateName, creatorID)
	if err != nil {
		t.Fatalf("error creating confirmation: %s", err)
	}
	return c
}

// MustConfirmation is a helper for tests that fails the test when
// confirmation creation fails.
func MustConfirmationWithContext(t *testing.T, theType Type,
	templateName TemplateName, creatorID string, data interface{}) *Confirmation {

	c, err := NewConfirmationWithContext(theType, templateName, creatorID, data)
	if err != nil {
		t.Fatalf("error creating confirmation: %s", err)
	}
	return c
}
