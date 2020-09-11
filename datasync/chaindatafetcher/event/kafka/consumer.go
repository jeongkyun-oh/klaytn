package kafka

import (
	"context"

	"github.com/Shopify/sarama"
)

type Consumer struct {
	cancel   chan bool
	handler  map[string]func(*sarama.ConsumerMessage) error
	consumer sarama.ConsumerGroup
	ctx      context.Context
	isActive bool
}

func NewConsumer(ctx context.Context, consumer sarama.ConsumerGroup) *Consumer {
	return &Consumer{
		cancel:   make(chan bool),
		handler:  map[string]func(*sarama.ConsumerMessage) error{},
		ctx:      ctx,
		consumer: consumer,
		isActive: false,
	}

}

func (r *Consumer) Subscribe(topic string, handler func(*sarama.ConsumerMessage) error) error {
	if r.handler[topic] != nil {
		return nil
	}
	r.handler[topic] = handler

	if r.isActive {
		r.cancel <- true
	}
	go func() {
		defer r.consumer.Close()
		topics := []string{}
		for topic := range r.handler {
			topics = append(topics, topic)
		}
		h := func(err chan<- error) {
			err <- r.consumer.Consume(r.ctx, topics, r)
		}
		res := make(chan error, 1)
		for {
			go h(res)
			select {
			case err := <-res:
				if err != nil {
					logger.Error("response error", "err", err)
				}
			case <-r.cancel:
				return
			case <-r.ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (r *Consumer) Setup(sess sarama.ConsumerGroupSession) error {
	logger.Info("consumer was initialized as %s", sess.MemberID)
	r.isActive = true
	return nil
}

func (r *Consumer) Cleanup(sess sarama.ConsumerGroupSession) error {
	logger.Info("consumer was cleaned up: %s", sess.MemberID)
	r.isActive = false
	return nil
}

func (r *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		logger.Debug("Message claimed: value = %s, timestamp = %v, topic = %s", string(message.Value), message.Timestamp, message.Topic)
		go r.handler[message.Topic](message)
		// session.MarkMessage(message, "")
	}

	return nil
}
