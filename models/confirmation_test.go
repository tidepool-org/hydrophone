package models

import (
	"testing"
)

func TestConfirmation(t *testing.T) {

	user, creator := "user@test.org", "123xf456"

	confirmation, _ := NewConfirmation(TypePasswordReset, user, creator)

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

	if confirmation.ToUser != user {
		t.Fatalf("The user should be [%s] but is [%s]", user, confirmation.ToUser)
	}

	if confirmation.CreatorId != creator {
		t.Fatalf("The creator should be [%s] but is [%s]", creator, confirmation.CreatorId)
	}

}

func TestConfirmation_NoCreator(t *testing.T) {

	user := "user@test.org"

	confirmation, _ := NewConfirmation(TypeCareteamInvite, user, "")

	if confirmation.Type != TypeCareteamInvite {
		t.Fatalf("The type should be [%s] but is [%s]", TypeCareteamInvite, confirmation.Type)
	}

	if confirmation.CreatorId != "" {
		t.Fatalf("The creator should be empty but is [%s]", confirmation.CreatorId)
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
