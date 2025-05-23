package infrastructure

import (
	"os"

	log "github.com/sirupsen/logrus"
)

// MustGetConfigFromEnvironment gets the value of the given variable and returns it if it's not empty
// Panic if the variable is empty (exit 1)
func MustGetConfigFromEnvironment(varName string) string {
	val := os.Getenv(varName)
	if val == "" {
		log.Fatalf("environment variable %s is not set", varName)
	}
	return val
}
