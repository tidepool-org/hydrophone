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
	details := fmt.Sprintf("Send message with subject[%s] to %v", subject, to)
	log.Println(details)
	return 200, details
}
