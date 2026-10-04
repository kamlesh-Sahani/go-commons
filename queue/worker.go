package queue

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
)

// ConsumerHandler is the handler callback for incoming queue messages.
type ConsumerHandler func(ctx context.Context, body []byte) error

// StartConsumer starts a worker loop consuming from a designated queue.
func (c *RabbitClient) StartConsumer(ctx context.Context, queueName string, prefetchCount int, handler ConsumerHandler) error {
	if c.Conn == nil || c.Conn.IsClosed() {
		return fmt.Errorf("rabbitmq connection is closed")
	}

	ch, err := c.Conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open consumer channel: %w", err)
	}

	if prefetchCount <= 0 {
		prefetchCount = 10
	}
	if err := ch.Qos(prefetchCount, 0, false); err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to set prefetch QoS: %w", err)
	}

	msgs, err := ch.ConsumeWithContext(
		ctx,
		queueName,
		"",    // consumer tag (auto-generated)
		false, // autoAck: false (manual acknowledgment)
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to start consumer for %s: %w", queueName, err)
	}

	go func() {
		defer ch.Close()
		log.Printf("[RabbitMQ] Consumer started on queue: %s\n", queueName)

		for {
			select {
			case <-ctx.Done():
				log.Printf("[RabbitMQ] Context cancelled. Stopping consumer on %s\n", queueName)
				return
			case d, ok := <-msgs:
				if !ok {
					log.Printf("[RabbitMQ] Delivery channel closed for queue %s\n", queueName)
					return
				}

				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[RabbitMQ] Worker panic recovered on %s: %v\nStack: %s", queueName, r, string(debug.Stack()))
							_ = d.Nack(false, false)
						}
					}()

					if err := handler(ctx, d.Body); err != nil {
						log.Printf("[RabbitMQ] Error processing message on %s (redelivered=%v): %v\n", queueName, d.Redelivered, err)
						if d.Redelivered {
							// Prevent poison pill infinite requeue loop
							_ = d.Nack(false, false)
						} else {
							_ = d.Nack(false, true)
						}
					} else {
						_ = d.Ack(false)
					}
				}()
			}
		}
	}()

	return nil
}
