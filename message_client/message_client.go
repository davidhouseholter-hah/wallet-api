package message_client

import (
	"fmt"

	"context"

	"github.com/callistaenterprise/goblog/common/tracing"
	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

// IMessagingClient defines our interface for connecting and consuming messages.
type IMessagingClient interface {
	ConnectToBroker(connectionString string)
	Publish(msg []byte, exchangeName string, exchangeType string) error
	PublishOnQueue(msg []byte, queueName string) error
	PublishOnQueueWithContext(ctx context.Context, msg []byte, queueName string) error
	Subscribe(exchangeName string, exchangeType string, consumerName string, handlerFunc func(amqp.Delivery)) error
	Close()
}

// AmqpClient is our real implementation, encapsulates a pointer to an amqp.Connection
type AmqpClient struct {
	conn *amqp.Connection
}

// ConnectToBroker connects to an AMQP broker using the supplied connectionString.
func (m *AmqpClient) ConnectToBroker(connectionString string) {
	if connectionString == "" {
		panic("Cannot initialize connection to broker, connectionString not set. Have you initialized?")
	}

	var err error
	m.conn, err = amqp.Dial(fmt.Sprintf("%s/", connectionString))
	if err != nil {
		panic("Failed to connect to AMQP compatible broker at: " + connectionString)
	}
}

// Publish publishes a message to the named exchange.
func (m *AmqpClient) Publish(body []byte, eventName string, exchangeName string, exchangeType string) error {
	if m.conn == nil {
		panic("Tried to send message before connection was initialized. Don't do that.")
	}
	ch, err := m.conn.Channel() // Get a channel from the connection
	defer ch.Close()
	err = ch.ExchangeDeclare(
		exchangeName, // name of the exchange
		exchangeType, // type
		false,        // durable
		false,        // delete when complete
		false,        // internal
		false,        // noWait
		nil,          // arguments
	)
	failOnError(err, "Failed to register an Exchange")

	queue, err := ch.QueueDeclare( // Declare a queue that will be created if not exists with some args
		eventName, // our queue name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)

	err = ch.QueueBind(
		queue.Name,   // name of the queue
		exchangeName, // bindingKey
		exchangeName, // sourceExchange
		false,        // noWait
		nil,          // arguments
	)

	err = ch.Publish( // Publishes a message onto the queue.
		exchangeName, // exchange
		eventName,    // routing key      q.Name
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			Body: body, // Our JSON body as []byte
		})
	logrus.Infof("A message was sent: %v", string(body))
	return err
}

// Close closes the connection to the AMQP-broker, if available.
func (m *AmqpClient) Close() {
	if m.conn != nil {
		logrus.Infoln("Closing connection to AMQP broker")
		m.conn.Close()
	}
}

func buildMessage(ctx context.Context, body []byte) amqp.Publishing {
	publishing := amqp.Publishing{
		ContentType: "application/json",
		Body:        body, // Our JSON body as []byte
	}
	if ctx != nil {
		child := tracing.StartChildSpanFromContext(ctx, "messaging")
		defer child.Finish()
		var val = make(opentracing.TextMapCarrier)
		err := tracing.AddTracingToTextMapCarrier(child, val)
		if err != nil {
			logrus.Errorf("Error injecting span context: %v", err.Error())
		} else {
			publishing.Headers = tracing.CarrierToMap(val)
		}
	}
	return publishing
}
func failOnError(err error, msg string) {
	if err != nil {
		logrus.Errorf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}
