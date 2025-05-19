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

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		return
	}

	pauseQueueName := fmt.Sprintf("%s.%s", routing.PauseKey, username)
	_, _, err = pubsub.DeclareAndBind(dial, routing.ExchangePerilDirect, pauseQueueName, routing.PauseKey, pubsub.QueueTypeTransient)
	if err != nil {
		return
	}

	state := gamelogic.NewGameState(username)
	err = pubsub.SubscribeJSON(dial, routing.ExchangePerilDirect, pauseQueueName, routing.PauseKey, pubsub.QueueTypeTransient, handlerPause(state))
	if err != nil {
		panic(err)
	}

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		<-signalChan
		fmt.Println("Received an interrupt, stopping services")
		closer(dial)
	}()

	replLoop(state)
}

func closer(dial *amqp.Connection) {
	if dial.IsClosed() {
		return
	}

	if err := dial.Close(); err != nil {
		panic(err)
	}
}

func replLoop(state *gamelogic.GameState) {
	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		firstWord := input[0]

		switch {
		case firstWord == "spawn":
			if spawn(state, input) {
				continue
			}
		case firstWord == "move":
			if move(state, input) {
				continue
			}
		case firstWord == "status":
			state.CommandStatus()
		case firstWord == "help":
			gamelogic.PrintClientHelp()
		case firstWord == "spam":
			log.Println("Spamming not allowed yet!")
		case firstWord == "quit":
			gamelogic.PrintQuit()
			break
		default:
			log.Println("What ?!?")
		}
	}
}

func spawn(state *gamelogic.GameState, input []string) bool {
	if len(input) != 3 {
		log.Println("usage: spawn <unit> <location>")
		return true
	}

	unit := input[1]
	location := input[2]

	log.Printf("Spawning %s at %s\n", unit, location)
	if err := state.CommandSpawn(input); err != nil {
		log.Fatalln(err)
	}

	return false
}

func move(state *gamelogic.GameState, input []string) bool {
	if len(input) != 3 {
		log.Println("usage: move <location> <number_of_units>")
		return true
	}

	location := input[1]
	units := input[2]

	log.Printf("Moving %s at %s\n", units, location)
	if _, err := state.CommandMove(input); err != nil {
		log.Fatalln(err)
	}

	log.Printf("Moved units %s to %s\n", units, location)

	return false
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) {
	return func(state routing.PlayingState) {
		defer fmt.Print("> ")

		gs.HandlePause(state)
	}
}
