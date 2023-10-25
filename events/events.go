package events

import (
	"context"
	"time"

	"github.com/tidepool-org/go-common/events"
	"github.com/tidepool-org/hydrophone/clients"
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
	ctx, cancel := context.WithTimeout(context.Background(), deleteTimeout)
	defer cancel()
	if err := h.store.RemoveConfirmationsForUser(ctx, payload.UserID); err != nil {
		return err
	}
	return nil
}
