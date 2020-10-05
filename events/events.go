package events

import (
	"github.com/tidepool-org/go-common/events"
	"github.com/tidepool-org/hydrophone/clients"
	"log"
)

type handler struct {
	events.NoopUserEventsHandler

	store clients.StoreClient
}

var _ events.UserEventsHandler = &handler{}

func NewHandler(store clients.StoreClient) events.EventHandler {
	return events.NewUserEventsHandler(&handler{
		store: store,
	})
}

func (h *handler) HandleDeleteUserEvent(payload events.DeleteUserEvent) error {
	log.Printf("Deleting confirmations for user %v", payload.UserID)
	return h.store.RemoveConfirmationsForUser(payload.UserID)
}
