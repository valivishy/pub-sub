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
	"strconv"
	"time"
)

const connectionString = "amqp://guest:guest@localhost:5672/"
const errorFormat = "error: %s\n"

func main() {
	dial, err := amqp.Dial(connectionString)
	if err != nil {
		panic(err)
	}
	defer closer(dial)

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		panic(err)
	}

	state := gamelogic.NewGameState(username)

	if err = preparePauseQueue(username, dial, state); err != nil {
		panic(err)
	}

	moveChannel, err := prepareMoveQueue(state, dial)
	if err != nil {
		panic(err)
	}

	if err = prepareWarQueue(state, dial); err != nil {
		panic(err)
	}

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		<-signalChan
		fmt.Println("Received an interrupt, stopping services")
		closer(dial)
	}()

	replLoop(state, moveChannel)
}

func prepareMoveQueue(state *gamelogic.GameState, dial *amqp.Connection) (*amqp.Channel, error) {
	moveQueueName := fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, state.Player.Username)
	moveKey := fmt.Sprintf("%s.*", routing.ArmyMovesPrefix)

	channel, _, err := pubsub.DeclareAndBind(dial, routing.ExchangePerilTopic, moveQueueName, moveKey, pubsub.QueueTypeTransient)
	if err != nil {
		return nil, err
	}

	return channel, pubsub.SubscribeJSON(dial, routing.ExchangePerilTopic, moveQueueName, moveKey, pubsub.QueueTypeTransient, handlerMove(state, channel))
}

func preparePauseQueue(username string, dial *amqp.Connection, state *gamelogic.GameState) error {
	pauseQueueName := fmt.Sprintf("%s.%s", routing.PauseKey, username)
	_, _, err := pubsub.DeclareAndBind(dial, routing.ExchangePerilDirect, pauseQueueName, routing.PauseKey, pubsub.QueueTypeTransient)
	if err != nil {
		return err
	}

	return pubsub.SubscribeJSON(dial, routing.ExchangePerilDirect, pauseQueueName, routing.PauseKey, pubsub.QueueTypeTransient, handlerPause(state))
}

func prepareWarQueue(state *gamelogic.GameState, dial *amqp.Connection) error {
	warKey := fmt.Sprintf("%s.*", routing.WarRecognitionsPrefix)

	channel, _, err := pubsub.DeclareAndBind(dial, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, warKey, pubsub.QueueTypeDurable)
	if err != nil {
		return err
	}

	return pubsub.SubscribeJSON(dial, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, warKey, pubsub.QueueTypeDurable, handlerWar(state, channel))
}

func closer(dial *amqp.Connection) {
	if dial.IsClosed() {
		return
	}

	if err := dial.Close(); err != nil {
		panic(err)
	}
}

func replLoop(state *gamelogic.GameState, moveChannel *amqp.Channel) {
	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		firstWord := input[0]

		switch firstWord {
		case "spawn":
			if spawn(state, input) {
				continue
			}
		case "move":
			if move(state, moveChannel, input) {
				continue
			}
		case "status":
			state.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			log.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			return
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

func move(state *gamelogic.GameState, moveChannel *amqp.Channel, input []string) bool {
	if len(input) < 3 {
		log.Println("usage: move <location> <unit_1> <unit_2> ... ")
		return true
	}

	location := input[1]
	units := input[2:]

	log.Printf("Moving %s at %s\n", units, location)
	if _, err := state.CommandMove(input); err != nil {
		log.Fatalln(err)
	}

	var movedUnits []gamelogic.Unit

	for _, unit := range units {
		unitId, err := strconv.Atoi(unit)
		if err != nil {
			log.Fatalln(err)
		}
		movedUnits = append(movedUnits, state.Player.Units[unitId])
	}

	moveKey := fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, state.Player.Username)
	if err := pubsub.PublishJSON(moveChannel, routing.ExchangePerilTopic, moveKey, gamelogic.ArmyMove{
		Player:     state.Player,
		Units:      movedUnits,
		ToLocation: gamelogic.Location(location),
	}); err != nil {
		log.Fatalln(err)
	}

	log.Printf("Moved units %s to %s, and published successfully. \n", units, location)

	return false
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(state routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")

		gs.HandlePause(state)

		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, moveChannel *amqp.Channel) func(armyMove gamelogic.ArmyMove) pubsub.AckType {
	return func(armyMove gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")

		switch gs.HandleMove(armyMove) {
		case gamelogic.MoveOutComeSafe:

			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(
				moveChannel,
				routing.ExchangePerilTopic,
				routing.WarRecognitionsPrefix+"."+gs.GetUsername(),
				gamelogic.RecognitionOfWar{
					Attacker: armyMove.Player,
					Defender: gs.GetPlayerSnap(),
				},
			)
			if err != nil {
				fmt.Printf(errorFormat, err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			fallthrough
		default:
			return pubsub.NackDiscard
		}
	}
}

func handlerWar(gs *gamelogic.GameState, publishCh *amqp.Channel) func(dw gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(dw gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		warOutcome, winner, loser := gs.HandleWar(dw)
		switch warOutcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			err := publishGameLog(
				publishCh,
				gs.GetUsername(),
				fmt.Sprintf("%s won a war against %s", winner, loser),
			)
			if err != nil {
				fmt.Printf(errorFormat, err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			if err := publishGameLog(
				publishCh,
				gs.GetUsername(),
				fmt.Sprintf("%s won a war against %s", winner, loser),
			); err != nil {
				fmt.Printf(errorFormat, err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			err := publishGameLog(
				publishCh,
				gs.GetUsername(),
				fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser),
			)
			if err != nil {
				fmt.Printf(errorFormat, err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		}

		fmt.Println("error: unknown war outcome")
		return pubsub.NackDiscard
	}
}

func publishGameLog(publishCh *amqp.Channel, username, msg string) error {
	return pubsub.PublishGob(
		publishCh,
		routing.ExchangePerilTopic,
		routing.GameLogSlug+"."+username,
		routing.GameLog{
			Username:    username,
			CurrentTime: time.Now(),
			Message:     msg,
		},
	)
}
