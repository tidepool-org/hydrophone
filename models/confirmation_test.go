package models

import (
	//"encoding/json"
	"testing"
)

const USERID = "1234-555"

func TestConfirmation(t *testing.T) {

	confirmation, _ := NewConfirmation(TypePasswordReset, USERID)

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

	if confirmation.ToUser != USERID {
		t.Fatalf("The user should be [%s] but is [%s]", USERID, confirmation.ToUser)
	}

	if confirmation.CreatorId != "" {
		t.Fatalf("The creator should note be set by default but is [%s]", confirmation.CreatorId)
	}

	confirmation.UpdateStatus(StatusCompleted)

	if confirmation.Status != StatusCompleted {
		t.Fatalf("Status should be [%s] but is [%s]", StatusCompleted, confirmation.Status)
	}

	if confirmation.Modified.IsZero() != false {
		t.Fatal("The modified time should have been set")
	}

}

func TestConfirmation_Context(t *testing.T) {

	type Extras struct {
		Blah  string `json:"blah"`
		Email string `json:"email"`
	}

	var jsonData = []byte(`{"blah": "stuff", "email": "test@user.org"}`)

	confirmation, _ := NewConfirmation(TypePasswordReset, USERID)

	confirmation.Context = jsonData

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
