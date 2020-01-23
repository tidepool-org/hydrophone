package clients

type (
	// NullNotifier
	NullNotifier struct {
	}
)

//NewSmtpNotifier creates a new SMTP notifier (using standard smtp to send emails)
func NewNullNotifier() (*NullNotifier, error) {
	return &NullNotifier{}, nil
}

// Send a message to a list of recipients with a given subject
func (c *NullNotifier) Send(to []string, subject string, msg string) (int, string) {
	return 200, "OK"
}
