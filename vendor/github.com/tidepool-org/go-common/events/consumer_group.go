package events

import (
	"context"
	"log"
	"sync"

	"github.com/Shopify/sarama"

	"github.com/tidepool-org/go-common/errors"
)

var ErrConsumerStopped = errors.New("consumer has been stopped")

// SaramaEventConsumer implements EventConsumer to consume messages received
// via Sarama consumer groups.
type SaramaEventConsumer struct {
	config   *CloudEventsConfig
	consumer MessageConsumer
	topic    string

	cancelFuncMu sync.Mutex
	cancelFunc   context.CancelFunc
}

func NewSaramaConsumerGroup(config *CloudEventsConfig, consumer MessageConsumer) (*SaramaEventConsumer, error) {
	if err := validateConsumerConfig(config); err != nil {
		return nil, err
	}
	if err := consumer.Initialize(config); err != nil {
		return nil, err
	}

	return &SaramaEventConsumer{
		config:   config,
		consumer: consumer,
		topic:    config.GetPrefixedTopic(),
	}, nil
}

func validateConsumerConfig(config *CloudEventsConfig) error {
	if config.KafkaConsumerGroup == "" {
		return errors.New("consumer group cannot be empty")
	}
	return nil
}

func (s *SaramaEventConsumer) Start() error {

	ctx, cancel := s.newContext()
	defer cancel()

	cg, err := sarama.NewConsumerGroup(
		s.config.KafkaBrokers,
		s.config.KafkaConsumerGroup,
		s.config.SaramaConfig,
	)
	if err != nil {
		return err
	}

	handler := &SaramaMessageConsumer{s.consumer}
	for {
		// `Consume` should be called inside an infinite loop, when a
		// server-side rebalance happens, the consumer session will need to be
		// recreated to get the new claims
		if err := cg.Consume(ctx, []string{s.topic}, handler); err != nil {
			log.Printf("Error from consumer: %v", err)
			if err == context.Canceled {
				return ErrConsumerStopped
			}
			return err
		}
	}
}

func (s *SaramaEventConsumer) Stop() error {
	s.cancelFuncMu.Lock()
	defer s.cancelFuncMu.Unlock()

	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil
	}
	return nil
}

func (s *SaramaEventConsumer) newContext() (context.Context, context.CancelFunc) {
	s.cancelFuncMu.Lock()
	defer s.cancelFuncMu.Unlock()
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	return ctx, cancel
}

// SaramaMessageConsumer implements sarama.ConsumerGroupHandler.
//
// It adapts a MessageConsumer for this purpose.
type SaramaMessageConsumer struct {
	MessageConsumer
}

// Cleanup implements sarama.ConsumerGroupHandler.sarama.ConsumeGroupHandler.
func (c *SaramaMessageConsumer) Cleanup(_ sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim implements sarama.ConsumerGroupHandler.sarama.ConsumeGroupHandler.
func (c *SaramaMessageConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		if err := c.HandleKafkaMessage(message); err != nil {
			log.Printf("failed to process kafka message: %v", err)
			return err
		}
		session.MarkMessage(message, "")
	}

	return nil
}

// Setup implements sarama.ConsumerGroupHandler.sarama.ConsumeGroupHandler.
func (c *SaramaMessageConsumer) Setup(_ sarama.ConsumerGroupSession) error {
	return nil
}
