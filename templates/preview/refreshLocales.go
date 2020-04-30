package main

import "log"

// Used to interact with a remote localization service such as Crowdin
type LocaleManager interface {
	DownloadLocales(localePath string) bool
}

// Default locale manager when none is used or implemented
type DefaultManager struct {
}

// Just print a message to stdout when it's called
func (l *DefaultManager) DownloadLocales(localesPath string) bool {
	log.Println("Faking locale download")
	return true
}
