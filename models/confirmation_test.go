package models

import (
	"strings"
	"testing"

	"github.com/mdblp/hydrophone/utils/otp"
)

const USERID = "1234-555"

type Extras struct {
	Blah  string `json:"blah"`
	Email string `json:"email"`
}

var contextData = &Extras{Blah: "stuff", Email: "test@user.org"}
var totp = &otp.TOTP{TimeStamp: 1594370515, OTP: "123456789"}

func contains(source string, compared string) bool {
	for _, char := range source {
		if !strings.Contains(compared, string(char)) {
			return false
		}
	}
	return true
}

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

	if confirmation.IsExpired() {
		t.Logf("the confirmation is not expired")
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

func Test_NewPatientPasswordResetConfirmation(t *testing.T) {

	confirmation, _ := NewConfirmation(TypePatientPasswordReset, TemplateNamePatientPasswordReset, USERID)

	if confirmation.Status != StatusPending {
		t.Fatalf("Status should be [%s] but is [%s]", StatusPending, confirmation.Status)
	}

	if confirmation.Key == "" {
		t.Fatal("There should be a generated key")
	}

	if confirmation.ShortKey == "" {
		t.Fatal("There should be a generated short key")
	}

	if confirmation.Created.IsZero() {
		t.Fatal("The created time should be set")
	}

	if confirmation.Modified.IsZero() == false {
		t.Fatal("The modified time should NOT be set")
	}

	if confirmation.Type != TypePatientPasswordReset {
		t.Fatalf("The type should be [%s] but is [%s]", TypePatientPasswordReset, confirmation.Type)
	}

	if confirmation.TemplateName != TemplateNamePatientPasswordReset {
		t.Fatalf("The template type should be [%s] but is [%s]", TemplateNamePatientPasswordReset, confirmation.TemplateName)
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

	if confirmation.GetReadableDuration() != "1" {
		t.Logf("expected `1` actual [%s]", confirmation.GetReadableDuration())
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

func Test_NewPatientPinResetConfirmation(t *testing.T) {

	confirmation, _ := NewConfirmationWithContext(TypePatientPinReset, TemplateNamePatientPinReset, USERID, totp)

	if confirmation.Status != StatusPending {
		t.Fatalf("Status should be [%s] but is [%s]", StatusPending, confirmation.Status)
	}

	if confirmation.Key == "" {
		t.Fatal("There should be a generated key")
	}

	decOTP := &otp.TOTP{}

	confirmation.DecodeContext(&decOTP)

	if decOTP.TimeStamp != totp.TimeStamp {
		t.Fatalf("context not decoded [%v]", decOTP)
	}

	if decOTP.OTP != totp.OTP {
		t.Fatalf("context not decoded [%v]", decOTP)
	}

	if confirmation.Created.IsZero() {
		t.Fatal("The created time should be set")
	}

	if confirmation.Modified.IsZero() == false {
		t.Fatal("The modified time should NOT be set")
	}

	if confirmation.Type != TypePatientPinReset {
		t.Fatalf("The type should be [%s] but is [%s]", TypePatientPinReset, confirmation.Type)
	}

	if confirmation.TemplateName != TemplateNamePatientPinReset {
		t.Fatalf("The template type should be [%s] but is [%s]", TemplateNamePatientPinReset, confirmation.TemplateName)
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

func Test_NewPatientPasswordInfoConfirmation(t *testing.T) {

	confirmation, _ := NewConfirmation(TypePatientPasswordInfo, TemplateNamePatientPasswordInfo, USERID)

	if confirmation.Status != StatusCompleted {
		t.Fatalf("Status should be [%s] but is [%s]", StatusCompleted, confirmation.Status)
	}

	if confirmation.Key == "" {
		t.Fatal("There should be a generated key")
	}

	if confirmation.ShortKey != "" {
		t.Fatal("There should not be a generated short key")
	}

	if confirmation.Created.IsZero() {
		t.Fatal("The created time should be set")
	}

	if confirmation.Modified.IsZero() == false {
		t.Fatal("The modified time should NOT be set")
	}

	if confirmation.Type != TypePatientPasswordInfo {
		t.Fatalf("The type should be [%s] but is [%s]", TypePatientPasswordInfo, confirmation.Type)
	}

	if confirmation.TemplateName != TemplateNamePatientPasswordInfo {
		t.Fatalf("The template type should be [%s] but is [%s]", TemplateNamePatientPasswordInfo, confirmation.TemplateName)
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

func TestConfirmationShortKey(t *testing.T) {

	keyLength := 8
	key, _ := generateShortKey(keyLength)

	if key == "" {
		t.Fatal("There should be a generated key")
	}

	if len(key) != keyLength {
		t.Fatal("The generated key should be 8 chars: ", len(key))
	}

	if !contains(key, letterBytes) {
		t.Fatal("The key should only contain authorized characters")
	}

}

func TestConfirmationTeam(t *testing.T) {

	confirmation, _ := NewConfirmation(TypeMedicalTeamPatientInvite, TemplateNameMedicalteamPatientInvite, USERID)

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

	if confirmation.Type != TypeMedicalTeamPatientInvite {
		t.Fatalf("The type should be [%s] but is [%s]", TypeMedicalTeamPatientInvite, confirmation.Type)
	}

	if confirmation.TemplateName != TemplateNameMedicalteamPatientInvite {
		t.Fatalf("The template type should be [%s] but is [%s]", TemplateNameMedicalteamPatientInvite, confirmation.TemplateName)
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
	if confirmation.GetReadableDuration() != "7" {
		t.Logf("expected `7` actual [%s]", confirmation.GetReadableDuration())
		t.Fail()
	}

}

func TestConfirmationMemberTeam(t *testing.T) {

	confirmation, _ := NewConfirmation(TypeMedicalTeamInvite, TemplateNameMedicalteamInvite, USERID)

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

	if confirmation.Type != TypeMedicalTeamInvite {
		t.Fatalf("The type should be [%s] but is [%s]", TypeMedicalTeamInvite, confirmation.Type)
	}

	if confirmation.TemplateName != TemplateNameMedicalteamInvite {
		t.Fatalf("The template type should be [%s] but is [%s]", TemplateNameMedicalteamInvite, confirmation.TemplateName)
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
	if confirmation.GetReadableDuration() != "7" {
		t.Logf("expected `7` actual [%s]", confirmation.GetReadableDuration())
		t.Fail()
	}
}

func TestConfirmationTeamDoAdmin(t *testing.T) {

	confirmation, _ := NewConfirmation(TypeMedicalTeamDoAdmin, TemplateNameMedicalteamDoAdmin, USERID)

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

	if confirmation.Type != TypeMedicalTeamDoAdmin {
		t.Fatalf("The type should be [%s] but is [%s]", TypeMedicalTeamDoAdmin, confirmation.Type)
	}

	if confirmation.TemplateName != TemplateNameMedicalteamDoAdmin {
		t.Fatalf("The template type should be [%s] but is [%s]", TemplateNameMedicalteamDoAdmin, confirmation.TemplateName)
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
}

func TestConfirmationCareteamInvite(t *testing.T) {

	confirmation, _ := NewConfirmation(TypeCareteamInvite, TemplateNameMedicalteamInvite, USERID)

	if confirmation.Status != StatusPending {
		t.Fatalf("Status should be [%s] but is [%s]", StatusPending, confirmation.Status)
	}

	if confirmation.Created.IsZero() {
		t.Fatal("The created time should be set")
	}

	if confirmation.Modified.IsZero() == false {
		t.Fatal("The modified time should NOT be set")
	}

	if confirmation.Type != TypeCareteamInvite {
		t.Fatalf("The type should be [%s] but is [%s]", TypeCareteamInvite, confirmation.Type)
	}

	if confirmation.TemplateName != TemplateNameMedicalteamInvite {
		t.Fatalf("The template type should be [%s] but is [%s]", TemplateNameMedicalteamInvite, confirmation.TemplateName)
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
	if confirmation.GetReadableDuration() != "7" {
		t.Logf("expected `7` actual [%s]", confirmation.GetReadableDuration())
		t.Fail()
	}
}

func TestConfirmationSignup(t *testing.T) {

	confirmation, _ := NewConfirmation(TypeSignUp, TemplateNameSignup, USERID)

	if confirmation.Status != StatusPending {
		t.Fatalf("Status should be [%s] but is [%s]", StatusPending, confirmation.Status)
	}

	if confirmation.Created.IsZero() {
		t.Fatal("The created time should be set")
	}

	if confirmation.Modified.IsZero() == false {
		t.Fatal("The modified time should NOT be set")
	}

	if confirmation.Type != TypeSignUp {
		t.Fatalf("The type should be [%s] but is [%s]", TypeSignUp, confirmation.Type)
	}

	if confirmation.TemplateName != TemplateNameSignup {
		t.Fatalf("The template type should be [%s] but is [%s]", TemplateNameSignup, confirmation.TemplateName)
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
	confirmationDuration := "31"
	if confirmation.GetReadableDuration() != confirmationDuration {
		t.Logf("expected `%s` actual [%s]", confirmationDuration, confirmation.GetReadableDuration())
		t.Fail()
	}

}
