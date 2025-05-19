package main

import (
	"fmt"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
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

	channel, err := dial.Channel()
	if err != nil {
		panic(err)
	}

	_, _, err = pubsub.DeclareAndBind(dial, routing.ExchangePerilTopic, routing.GameLogSlug, fmt.Sprintf("%s.*", routing.GameLogSlug), pubsub.QueueTypeDurable)
	if err != nil {
		panic(err)
	}

	gamelogic.PrintServerHelp()

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		<-signalChan
		fmt.Println("Received an interrupt, stopping services")
		closer(dial)
	}()

	replLoop(channel)
}

func replLoop(channel *amqp.Channel) {
	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		firstWord := input[0]
		switch {
		case firstWord == "pause":
			log.Println("Pausing the game")
			if err := publishPauseMessage(channel, true); err != nil {
				return
			}
		case firstWord == "resume":
			log.Println("Resuming the game")
			if err := publishPauseMessage(channel, false); err != nil {
				return
			}
		case firstWord == "quit":
			log.Println("Bye!")
			break
		default:
			log.Println("What ?!?")
		}
	}
}

func publishPauseMessage(channel *amqp.Channel, isPaused bool) error {
	return pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: isPaused})
}

func closer(dial *amqp.Connection) {
	if dial.IsClosed() {
		return
	}

	if err := dial.Close(); err != nil {
		panic(err)
	}
}
