package kafka

import (
	"context"

	"github.com/IBM/sarama"
)

type MessageHandler interface {
	Handle(ctx context.Context, msg *sarama.ConsumerMessage) error
}

type Consumer struct {
	group   sarama.ConsumerGroup
	handler MessageHandler
}

func NewConsumer(brokers []string, groupID string, cfg *sarama.Config, handler MessageHandler) (*Consumer, error) {
	if cfg == nil {
		cfg = sarama.NewConfig()
	}
	cfg.Version = sarama.V2_5_0_0
	g, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, err
	}
	return &Consumer{group: g, handler: handler}, nil
}

func (c *Consumer) Run(ctx context.Context, topics []string) error {
	for {
		if err := c.group.Consume(ctx, topics, consumerGroupHandler{handler: c.handler}); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func (c *Consumer) Close() error {
	return c.group.Close()
}

type consumerGroupHandler struct {
	handler MessageHandler
}

func (h consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h consumerGroupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		if err := h.handler.Handle(sess.Context(), message); err != nil {
			// retry/handling delegated to handler
			continue
		}
		sess.MarkMessage(message, "")
	}
	return nil
}
