package clients

import (
	"testing"
)

func TestPunycodeEmail(t *testing.T) {
	tests := []struct {
		InputEmail   string
		EncodedEmail string
	}{
		{"regular@email.com", "regular@email.com"},
		{"someone@mail.com", "someone@mail.com"},
		{"someone@måil.com", "someone@xn--mil-ula.com"},
		{`"funky@but@valid$email"@site.com`, `"funky@but@valid$email"@site.com`},
		{`silly\@email@g∞gl€.com`, `silly\@email@xn--ggl-m50au1g.com`},
	}

	for _, test := range tests {
		encoded, err := punycodeEmail(test.InputEmail)
		if err != nil {
			t.Errorf(`Error punycoding "%s": %v`, test.InputEmail, err)
			continue
		}
		if encoded != test.EncodedEmail {
			t.Errorf(`Expected "%s" to be encoded as "%s", got "%s"`, test.InputEmail, test.EncodedEmail, encoded)
		}
	}
}
