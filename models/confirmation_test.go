package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

const USERID = "1234-555"

type Extras struct {
	Blah  string `json:"blah"`
	Email string `json:"email"`
}

var contextData = &Extras{Blah: "stuff", Email: "test@user.org"}

func Test_NewConfirmation(t *testing.T) {

	confirmation, _ := NewConfirmation(TypePasswordReset, TemplateNamePasswordReset, USERID)

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

	confirmation.UpdateStatus(StatusCompleted)

	if confirmation.Status != StatusCompleted {
		t.Fatalf("Status should be [%s] but is [%s]", StatusCompleted, confirmation.Status)
	}

	if confirmation.Modified.IsZero() != false {
		t.Fatal("The modified time should have been set")
	}

}

func Test_NewConfirmationWithContext(t *testing.T) {

	confirmation, _ := NewConfirmationWithContext(TypePasswordReset, TemplateNamePasswordReset, USERID, contextData)

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

	confirmation, _ := NewConfirmation(TypePasswordReset, TemplateNamePasswordReset, USERID)

	confirmation.AddContext(contextData)

	myExtras := &Extras{}

	confirmation.DecodeContext(&myExtras)

	if myExtras.Blah == "" {
		t.Fatalf("context not decoded [%v]", myExtras)
	}

	if myExtras.Email == "" {
		t.Fatalf("context not decoded [%v]", myExtras)
	}

}

func TestConfirmationKey(t *testing.T) {

	key, _ := generateKey()

	if key == "" {
		t.Fatal("There should be a generated key")
	}

	if len(key) != 32 {
		t.Fatal("The generated key should be 32 chars: ", len(key))
	}
}

func TestDurationMinutes(s *testing.T) {
	s.Run("parses 10", func(t *testing.T) {
		d := DurationMinutes(0)
		if err := d.UnmarshalJSON([]byte(`42`)); err != nil {
			t.Fatalf("expected nil, got %+v", err)
		}
		if dur := d.Duration(); dur != 42*time.Minute {
			t.Fatalf("expected 42 minutes, got %s", dur)
		}
	})
	s.Run("parses 0", func(t *testing.T) {
		d := DurationMinutes(3 * time.Minute)
		if err := d.UnmarshalJSON([]byte(`0`)); err != nil {
			t.Fatalf("expected nil, got %+v", err)
		}
		if dur := d.Duration(); dur != 0*time.Minute {
			t.Fatalf("expected 0 minutes, got %s", dur)
		}
	})
	s.Run("parses null as 0 minutes", func(t *testing.T) {
		d := DurationMinutes(0)
		if err := d.UnmarshalJSON([]byte(`null`)); err != nil {
			t.Fatalf("expected nil, got %+v", err)
		}
		if dur := d.Duration(); dur != 0*time.Minute {
			t.Fatalf("expected 0 minutes, got %s", dur)
		}
	})
	s.Run("parses an empty value as 0 minutes", func(t *testing.T) {
		d := DurationMinutes(0)
		if err := d.UnmarshalJSON([]byte(``)); err != nil {
			t.Fatalf("expected nil, got %+v", err)
		}
		if dur := d.Duration(); dur != 0*time.Minute {
			t.Fatalf("expected 0 minutes, got %s", dur)
		}
	})
}

func TestThresholdValidateUnits(s *testing.T) {
	s.Run("accepts mg/dL", func(t *testing.T) {
		raw := []byte(fmt.Sprintf(`{"units":%q,"value":42}`, UnitsMilligramsPerDeciliter))
		threshold := &Threshold{}
		if err := json.Unmarshal(raw, threshold); err != nil {
			t.Fatalf("expected nil, got %s", err)
		}
		if threshold.Value != 42 {
			t.Fatalf("expected 42, got %f", threshold.Value)
		}
		if threshold.Units != UnitsMilligramsPerDeciliter {
			t.Fatalf("expected %q, got %q", UnitsMilligramsPerDeciliter, threshold.Units)
		}
	})
	s.Run("accepts mmol/L", func(t *testing.T) {
		raw := []byte(fmt.Sprintf(`{"units":%q,"value":42}`, UnitsMillimollsPerLiter))
		threshold := &Threshold{}
		if err := json.Unmarshal(raw, threshold); err != nil {
			t.Fatalf("expected nil, got %s", err)
		}
		if threshold.Value != 42 {
			t.Fatalf("expected 42, got %f", threshold.Value)
		}
		if threshold.Units != UnitsMillimollsPerLiter {
			t.Fatalf("expected %q, got %q", UnitsMillimollsPerLiter, threshold.Units)
		}
	})
	s.Run("doesn't accept lb/gal", func(t *testing.T) {
		lbPerGal := "lb/gal"
		raw := []byte(fmt.Sprintf(`{"units":%q,"value":42}`, lbPerGal))
		threshold := &Threshold{}
		err := json.Unmarshal(raw, threshold)
		if errors.Is(err, nil) {
			t.Fatalf("expected validation error, got nil")
		}
	})
	s.Run("doesn't accept blank Units", func(t *testing.T) {
		raw := []byte(fmt.Sprintf(`{"units":"","value":42}`))
		threshold := &Threshold{}
		err := json.Unmarshal(raw, threshold)
		if errors.Is(err, nil) {
			t.Fatalf("expected validation error, got nil")
		}
	})
	s.Run("is case-sensitive", func(t *testing.T) {
		badUnits := strings.ToUpper(UnitsMillimollsPerLiter)
		raw := []byte(fmt.Sprintf(`{"units":%q,"value":42}`, badUnits))
		threshold := &Threshold{}
		err := json.Unmarshal(raw, threshold)
		if errors.Is(err, nil) {
			t.Fatalf("expected validation error, got nil")
		}
	})
}
