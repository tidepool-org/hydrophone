package events

import (
	"context"
	"github.com/tidepool-org/go-common/events"
	"github.com/tidepool-org/hydrophone/clients"
	"log"
	"time"
)

const deleteTimeout = 60 * time.Second

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
	ctx, _ := context.WithTimeout(context.Background(), deleteTimeout)
	return h.store.RemoveConfirmationsForUser(ctx, payload.UserID)
}
