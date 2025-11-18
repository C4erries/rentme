package kafka

import (
	"context"

	"github.com/IBM/sarama"
)

type Producer struct {
	sync sarama.SyncProducer
}

func NewProducer(brokers []string, cfg *sarama.Config) (*Producer, error) {
	if cfg == nil {
		cfg = sarama.NewConfig()
	}
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Idempotent = true
	cfg.Producer.Return.Successes = true
	sync, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		return nil, err
	}
	return &Producer{sync: sync}, nil
}

func (p *Producer) Publish(ctx context.Context, topic string, key string, payload []byte, headers map[string]string) error {
	var hs []sarama.RecordHeader
	for k, v := range headers {
		hs = append(hs, sarama.RecordHeader{Key: []byte(k), Value: []byte(v)})
	}
	msg := &sarama.ProducerMessage{
		Topic:   topic,
		Key:     sarama.StringEncoder(key),
		Value:   sarama.ByteEncoder(payload),
		Headers: hs,
	}
	_, _, err := p.sync.SendMessage(msg)
	return err
}

func (p *Producer) Close() error {
	if p.sync == nil {
		return nil
	}
	return p.sync.Close()
}
