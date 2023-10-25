package clients

import (
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

type (
	MockNotifier struct{}
)

func NewMockNotifier() Notifier {
	return &MockNotifier{}
}

func (c *MockNotifier) Send(to []string, subject string, msg string) (int, string) {
	details := fmt.Sprintf("Send subject[%s] with message[%s] to %v", subject, msg, to)
	zap.S().Info(details)
	return 200, details
}

// MockNotifierModule is a fx module for this component
var MockNotifierModule = fx.Options(
	fx.Provide(NewMockNotifier),
)
