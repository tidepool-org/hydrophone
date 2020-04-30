package localize

import "fmt"

type MockLocalizer struct {
	translations map[string]string
}

func NewMockLocalizer(translations map[string]string) *MockLocalizer {
	return &MockLocalizer{
		translations: translations,
	}
}

// getLocalizedContentPart returns translated content part based on key and locale
func (l *MockLocalizer) Localize(key string, locale string, data map[string]interface{}) (string, error) {
	if msg, ok := l.translations[key]; !ok {
		return "", fmt.Errorf("failed to localize %s", key)
	} else {
		return msg, nil
	}
}
