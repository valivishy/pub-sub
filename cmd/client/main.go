package main

import (
	"fmt"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
	"os"
	"os/signal"
)

const connectionString = "amqp://guest:guest@localhost:5672/"

func main() {
	dial, err := amqp.Dial(connectionString)
	if err != nil {
		panic(err)
	}
	defer closer(dial)

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		return
	}

	_, _, err = pubsub.DeclareAndBind(dial, routing.ExchangePerilDirect, fmt.Sprintf("%s.%s", routing.PauseKey, username), routing.PauseKey, pubsub.QueueTypeTransient)
	if err != nil {
		return
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	fmt.Println("Received an interrupt, stopping services")
	closer(dial)
}

func closer(dial *amqp.Connection) {
	if dial.IsClosed() {
		return
	}

	if err := dial.Close(); err != nil {
		panic(err)
	}
}
