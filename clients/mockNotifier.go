package clients

import (
	"fmt"
	"log"

	"go.uber.org/fx"
)

type (
	MockNotifier struct{}
)

func NewMockNotifier() Notifier {
	return &MockNotifier{}
}

func (c *MockNotifier) Send(to []string, subject string, msg string) (int, string) {
	details := fmt.Sprintf("Send subject[%s] with message[%s] to %v", subject, msg, to)
	log.Println(details)
	return 200, details
}

//MockNotifierModule is a fx module for this component
var MockNotifierModule = fx.Options(
	fx.Provide(NewMockNotifier),
)
