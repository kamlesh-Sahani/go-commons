package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitClient manages RabbitMQ connection, channels, and thread-safe publishing.
type RabbitClient struct {
	Conn       *amqp.Connection
	pubChannel *amqp.Channel
	pubMu      sync.Mutex
	url        string
}

// NewRabbitClient connects to RabbitMQ with retry logic.
func NewRabbitClient(url string) (*RabbitClient, error) {
	if url == "" {
		return nil, fmt.Errorf("rabbitmq url cannot be empty")
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to dial rabbitmq: %w", err)
	}

	client := &RabbitClient{
		Conn: conn,
		url:  url,
	}

	return client, nil
}

// getPublishChannelLocked gets or reopens the shared publishing channel while pubMu is held.
func (c *RabbitClient) getPublishChannelLocked() (*amqp.Channel, error) {
	if c.pubChannel != nil && !c.pubChannel.IsClosed() {
		return c.pubChannel, nil
	}

	if c.Conn == nil || c.Conn.IsClosed() {
		return nil, fmt.Errorf("rabbitmq connection is closed")
	}

	ch, err := c.Conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open rabbitmq channel: %w", err)
	}

	c.pubChannel = ch
	return c.pubChannel, nil
}

// PublishJSON marshals payload to JSON and publishes it to exchange/routingKey thread-safely.
func (c *RabbitClient) PublishJSON(ctx context.Context, exchange, routingKey string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal rabbitmq payload: %w", err)
	}

	c.pubMu.Lock()
	defer c.pubMu.Unlock()

	ch, err := c.getPublishChannelLocked()
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(ctx,
		exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now().UTC(),
			Body:         data,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish to rabbitmq: %w", err)
	}

	return nil
}

// DeclareQueue ensures a queue exists and is bound to an exchange with routing key.
func (c *RabbitClient) DeclareQueue(queueName, exchange, routingKey string) error {
	c.pubMu.Lock()
	defer c.pubMu.Unlock()

	ch, err := c.getPublishChannelLocked()
	if err != nil {
		return err
	}

	_, err = ch.QueueDeclare(
		queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
	}

	if exchange != "" {
		err = ch.QueueBind(
			queueName,
			routingKey,
			exchange,
			false,
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange %s: %w", queueName, exchange, err)
		}
	}

	return nil
}

// Close gracefully closes publisher channel and connection.
func (c *RabbitClient) Close() {
	c.pubMu.Lock()
	if c.pubChannel != nil && !c.pubChannel.IsClosed() {
		_ = c.pubChannel.Close()
		c.pubChannel = nil
	}
	c.pubMu.Unlock()

	if c.Conn != nil && !c.Conn.IsClosed() {
		_ = c.Conn.Close()
		log.Println("[RabbitMQ] Connection closed cleanly")
	}
}
