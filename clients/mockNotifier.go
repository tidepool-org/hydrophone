package clients

import (
	"fmt"
	"log"
)

type (
	MockNotifier struct{}
)

func NewMockNotifier() *MockNotifier {
	return &MockNotifier{}
}

func (c *MockNotifier) Send(to []string, subject string, msg string) (string, error) {
	details := fmt.Sprintf("Send subject[%s] with message[%s] to %v", subject, msg, to)
	log.Println(details)
	return details, nil
}
