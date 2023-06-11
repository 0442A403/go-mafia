package main

import (
	"bufio"
	"context"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	pb "0442a403.hse.ru/mafia/pkg/proto/mafia-server"
	"google.golang.org/grpc"
)

var (
	addr         = flag.String("addr", "localhost:50051", "the address to connect to")
	create_lobby = flag.Bool("create-lobby", false, "create new lobby")
	connect_to   = flag.String("connect-to", "", "create new lobby")
	debug        = flag.Bool("debug", false, "print debug messages")
	name         = flag.String("name", "Unnamed", "your nickname")
)

func main() {
	flag.Parse()
	if !*create_lobby && len(*connect_to) == 0 {
		log.Fatalf("You have to set create-lobby or connect-to option")
	}

	conn, err := grpc.Dial(*addr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewMafiaEngineClient(conn)

	ctx := context.Background()

	lobby_id := *connect_to
	if *create_lobby {
		response, err := c.CreateLobby(ctx, &pb.CreateLobbyRequest{PlayerName: *name})
		if err != nil {
			log.Fatalf("could not create: %v", err)
		}
		log.Printf("Lobby %s created. To invite your friend send him this id.", response.GetLobbyId())

		lobby_id = response.GetLobbyId()
	} else {
		_, err := c.ConnectLobby(ctx, &pb.ConnectLobbyRequest{
			LobbyId:    lobby_id,
			PlayerName: *name,
		})
		if err != nil {
			log.Fatalf("could not create: %v", err)
		}
		log.Printf("Connected to %s lobby.", lobby_id)
	}

	go func() {
		for {
			state, err := c.GetState(ctx, &pb.GetStateRequest{LobbyId: lobby_id})
			if err != nil {
				log.Fatalf("Can't get state: %v", err)
			}

			if *debug {
				log.Printf("Got state: %s", state.String())
			}

			for _, notification := range state.Notifications {
				log.Print(notification)
			}

			time.Sleep(time.Second * 5)
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		log.Printf("New string: %s", scanner.Text())

		args := strings.Split(scanner.Text(), " ")

		switch args[0] {
		case "start":
			bot_number := 0
			if len(args) >= 2 {
				x, err := strconv.Atoi(args[1])
				if err != nil {
					return
				}
				bot_number = x
			}

			_, err := c.StartGame(ctx, &pb.StartGameRequest{
				LobbyId:   lobby_id,
				BotNumber: int32(bot_number)})
			if err != nil {
				log.Fatalf("Can't start game %v", err)
			}
		}
	}
}
