package queue

import (
	"context"
	"testing"
)

func TestRabbitClient_Validation(t *testing.T) {
	t.Run("Empty URL returns error", func(t *testing.T) {
		_, err := NewRabbitClient("")
		if err == nil {
			t.Errorf("expected error for empty URL, got nil")
		}
	})

	t.Run("Closed connection returns error on publish", func(t *testing.T) {
		client := &RabbitClient{
			url: "amqp://localhost:5672",
		}
		err := client.PublishJSON(context.Background(), "test_exchange", "test.key", map[string]string{"foo": "bar"})
		if err == nil {
			t.Errorf("expected error publishing on closed connection, got nil")
		}
	})

	t.Run("DeclareQueue on closed connection returns error", func(t *testing.T) {
		client := &RabbitClient{
			url: "amqp://localhost:5672",
		}
		err := client.DeclareQueue("test_queue", "test_exchange", "test.key")
		if err == nil {
			t.Errorf("expected error declaring queue on closed connection, got nil")
		}
	})

	t.Run("StartConsumer on closed connection returns error", func(t *testing.T) {
		client := &RabbitClient{
			url: "amqp://localhost:5672",
		}
		err := client.StartConsumer(context.Background(), "test_queue", 10, func(ctx context.Context, body []byte) error {
			return nil
		})
		if err == nil {
			t.Errorf("expected error starting consumer on closed connection, got nil")
		}
	})
}
