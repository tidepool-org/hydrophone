package clients

import (
	"fmt"
	"log"
)

type (
	MockNotifier struct {
		lastSentEmailsArgs *EmailArgs
	}

	EmailArgs struct {
		To      []string
		Subject string
		Msg     string
	}
)

func NewMockNotifier() *MockNotifier {
	return &MockNotifier{}
}

func (c *MockNotifier) Send(to []string, subject string, msg string, tags map[string]string) (int, string) {
	details := fmt.Sprintf("Send message with subject[%s] to %v", subject, to)
	c.lastSentEmailsArgs = &EmailArgs{To: to, Subject: subject, Msg: msg}
	log.Println(details)
	return 200, details
}

func (c *MockNotifier) GetLastEmailSubject() string {
	return c.lastSentEmailsArgs.Subject
}
