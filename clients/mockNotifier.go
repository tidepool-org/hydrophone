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

func (c *MockNotifier) Send(to []string, subject string, msg string) (int, string) {
	details := fmt.Sprintf("Send subject[%s] with message[%s] to %v", subject, msg, to)
	log.Println(details)
	return 200, details
}
