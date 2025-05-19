package pubsub

import (
	"context"
	"encoding/json"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
)

const (
	QueueTypeDurable   int = 1
	QueueTypeTransient int = 2
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	valBytes, err := json.Marshal(val)
	if err != nil {
		return err
	}

	if err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        valBytes,
	}); err != nil {
		return err
	}

	return nil
}

func DeclareAndBind(conn *amqp.Connection, exchange, queueName, key string, simpleQueueType int) (*amqp.Channel, amqp.Queue, error) {
	channel, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	durable := simpleQueueType == QueueTypeDurable

	queue, err := channel.QueueDeclare(queueName, durable, !durable, !durable, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	err = channel.QueueBind(queueName, key, exchange, false, nil)

	return channel, queue, err
}

func SubscribeJSON[T any](conn *amqp.Connection, exchange, queueName, key string, simpleQueueType int, handler func(T)) error {
	channel, queue, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}

	consume, err := channel.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for delivery := range consume {
			var target T
			err = json.Unmarshal(delivery.Body, &target)
			if err != nil {
				log.Fatal(err)
			}

			handler(target)
			err = delivery.Ack(true)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	return nil
}
