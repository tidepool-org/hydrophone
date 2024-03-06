package events

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/tidepool-org/go-common/events"
	"github.com/tidepool-org/hydrophone/clients"
)

const deleteTimeout = 60 * time.Second

type handler struct {
	events.NoopUserEventsHandler

	store  clients.StoreClient
	logger *zap.SugaredLogger
}

var _ events.UserEventsHandler = &handler{}

func NewHandler(store clients.StoreClient, logger *zap.SugaredLogger) events.EventHandler {
	return events.NewUserEventsHandler(&handler{
		store:  store,
		logger: logger,
	})
}

func (h *handler) HandleDeleteUserEvent(payload events.DeleteUserEvent) error {
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), deleteTimeout)
	defer cancel()
	defer func(err *error) {
		log := h.logger.With(zap.String("userId", payload.UserID))
		if err != nil {
			log.With(zap.Error(*err)).Error("deleting confirmations")
		} else {
			log.With().Info("successfully deleted confirmations")
		}
	}(&err)
	if err = h.store.RemoveConfirmationsForUser(ctx, payload.UserID); err != nil {
		return err
	}
	return nil
}
