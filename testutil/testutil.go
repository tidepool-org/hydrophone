package testutil

import (
	"testing"

	"go.uber.org/zap"
)

func NewLogger(t *testing.T) *zap.SugaredLogger {
	config := zap.NewDevelopmentConfig()
	logger, err := config.Build()
	if err != nil {
		t.Logf("error configuring zap logger: %s", err)
		logger = zap.NewNop()
	}
	return logger.Sugar()
}
