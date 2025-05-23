package models

// UserData is the data structure returned from a successful Login query.
type UserData struct {
	UserID         string   `json:"userid,omitempty" bson:"userid,omitempty"` // map userid to id
	Username       string   `json:"username,omitempty" bson:"username,omitempty"`
	Emails         []string `json:"emails,omitempty" bson:"emails,omitempty"`
	PasswordExists bool     `json:"passwordExists,omitempty"` // Does a password exist for the user?
	Roles          []string `json:"roles,omitempty" bson:"roles,omitempty"`
	TermsAccepted  string   `json:"termsAccepted,omitempty" bson:"termsAccepted,omitempty"`
	EmailVerified  bool     `json:"emailVerified" bson:"authenticated"` //tag is name `authenticated` for historical reasons
	IdVerified     bool     `json:"idVerified"`
}

// UserUpdate is the data structure for updating of a users details
type UserUpdate struct {
	Username      *string   `json:"username,omitempty"`
	Emails        *[]string `json:"emails,omitempty"`
	Password      *string   `json:"password,omitempty"`
	Roles         *[]string `json:"roles,omitempty"`
	EmailVerified *bool     `json:"emailVerified,omitempty"`
}

func (u *UserData) HasRole(role string) bool {
	for _, userRole := range u.Roles {
		if userRole == role {
			return true
		}
	}
	return false
}
