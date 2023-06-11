package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	. "0442a403.hse.ru/mafia/engine/common"
	"0442a403.hse.ru/mafia/engine/utils"
	pb "0442a403.hse.ru/mafia/pkg/proto/mafia-server"

	"github.com/google/uuid"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedMafiaEngineServer
}

type ip_info struct {
	ip        Ip
	lobby_id  LobbyId
	player_id PlayerId
}

var (
	port     = flag.Int("port", 50051, "The server port")
	ip_infos = make(map[Ip]ip_info)
	lobbies  = make(map[LobbyId]Lobby)
)

func (server) CreateLobby(ctx context.Context, in *pb.CreateLobbyRequest) (*pb.CreateLobbyReply, error) {
	player_ip := utils.ParseIp(ctx)
	log.Printf("Received CreateRequest from %s", player_ip)

	if _, ok := ip_infos[player_ip]; ok {
		return nil, status.Error(400, "Player is already connected to lobby")
	}

	lobby_id := LobbyId(uuid.NewString())
	player_id := PlayerId(uuid.NewString())
	ip_infos[player_ip] = ip_info{
		ip:        player_ip,
		lobby_id:  lobby_id,
		player_id: player_id,
	}
	lobbies[lobby_id] = Lobby{
		LobbyId:   lobby_id,
		CreatorId: player_id,
		Players: []Player{
			{
				Id:      player_id,
				Ip:      player_ip,
				Name:    in.GetPlayerName(),
				LobbyId: lobby_id,
			},
		},
	}

	log.Printf("Lobby %s created", lobby_id)

	return &pb.CreateLobbyReply{LobbyId: string(lobby_id)}, nil
}

func (server) ConnectLobby(ctx context.Context, in *pb.ConnectLobbyRequest) (*pb.ConnectLobbyReply, error) {
	player_ip := utils.ParseIp(ctx)
	log.Printf("Received ConnectLobby from %s", player_ip)

	if ip_info, ok := ip_infos[player_ip]; ok && ip_info.lobby_id == LobbyId(in.LobbyId) {
		return &pb.ConnectLobbyReply{}, nil
	}

	if _, ok := ip_infos[player_ip]; ok {
		return nil, status.Error(400, "You are connected to another lobby")
	}

	lobby, ok := lobbies[LobbyId(in.GetLobbyId())]
	if !ok {
		return nil, status.Error(400, "There is no such lobby")
	}

	if lobby.GameState != NOT_STARTED {
		return nil, status.Error(400, "This is already started")
	}

	if len(in.GetPlayerName()) == 0 {
		return nil, status.Error(400, "Name cannot be empty")
	}

	for i := 0; i < len(lobby.Players); i++ {
		lobby.Players[i].Notifications = append(lobby.Players[i].Notifications,
			fmt.Sprintf("Player %s has joined", in.PlayerName))
	}

	player_id := PlayerId(uuid.NewString())
	player := Player{
		Id:      player_id,
		Ip:      player_ip,
		Name:    in.PlayerName,
		LobbyId: lobby.LobbyId,
	}
	lobby.Players = append(lobby.Players, player)
	lobbies[LobbyId(in.GetLobbyId())] = lobby
	ip_infos[player_ip] = ip_info{
		ip:        player_ip,
		player_id: player_id,
		lobby_id:  lobby.LobbyId,
	}

	return &pb.ConnectLobbyReply{}, nil
}

func (server) GetState(ctx context.Context, in *pb.GetStateRequest) (*pb.GetStateReply, error) {
	player_ip := utils.ParseIp(ctx)
	log.Printf("Received ConnectLobby from %s", player_ip)

	ip_info, ok := ip_infos[player_ip]
	if !ok || ip_info.lobby_id != LobbyId(in.GetLobbyId()) {
		return nil, status.Error(400, "Player is not connected to that lobby")
	}

	lobby := lobbies[LobbyId(in.GetLobbyId())]
	request_player := lobby.FindPlayer(ip_info.player_id)

	response := &pb.GetStateReply{
		State: &pb.State{
			LobbyId:   in.GetLobbyId(),
			State:     pb.GameState(lobby.GameState), // TODO Change to switch function
			DayStage:  pb.DayStage(lobby.DayStage),   // TODO Change to switch function
			DayNumber: int32(lobby.DayNumber),
		},
	}

	for _, player := range lobby.Players {
		response.State.Players = append(response.State.Players,
			&pb.Player{
				PlayerId: string(player.Id),
				Name:     player.Name,
				IsDead:   player.IsDead,
				IsRobot:  player.IsRobot,
			})
	}
	for _, notification := range request_player.Notifications {
		response.Notifications = append(response.Notifications, &pb.Notification{
			Text: notification,
		})
	}
	request_player.Notifications = make([]string, 1)

	return response, nil
}

func (server) StartGame(ctx context.Context, in *pb.StartGameRequest) (*pb.StartGameReply, error) {
	player_ip := utils.ParseIp(ctx)
	log.Printf("Received start game %s", in.LobbyId)

	ip_info, ok := ip_infos[player_ip]
	if !ok || ip_info.lobby_id != LobbyId(in.LobbyId) {
		return nil, status.Error(400, "You are not connected to lobby")
	}

	lobby := lobbies[ip_info.lobby_id]
	if lobby.CreatorId != ip_info.player_id {
		return nil, status.Error(400, "Game can be started only by creator")
	}
	if lobby.GameState != NOT_STARTED {
		return nil, status.Error(400, "Game is already started")
	}
	if len(lobby.Players) < 1 { //TODO return 2 players
		return nil, status.Error(400, "There must be at least 2 real players")
	}

	lobby.GameState = RUNNING
	lobby.DayNumber = 1
	lobby.DayStage = DAY
	lobby.HumanNumber = len(lobby.Players)

	player_number := len(lobby.Players)
	mafia_index, militia_index := rand.Int()%player_number, rand.Int()%(player_number-1)
	if militia_index >= mafia_index {
		militia_index++
	}

	for i := 0; i < len(lobby.Players); i++ {
		var role PlayerRole
		switch i {
		case mafia_index:
			role = MAFIA
		case militia_index:
			role = MILITIA
		default:
			role = SEEKER
		}

		lobby.Players[i].Role = role
		lobby.Players[i].Notifications = append(lobby.Players[i].Notifications,
			fmt.Sprintf("Your role is %s", lobby.Players[i].Role.Stringify()))
	}

	for i := 0; i < int(in.BotNumber); i++ {
		player_id := PlayerId(uuid.NewString())
		lobby.Players = append(lobby.Players,
			Player{
				Id:      player_id,
				Name:    fmt.Sprintf("Bot #%d", i+1),
				LobbyId: lobby.LobbyId,
				IsRobot: true,
				Role:    SEEKER,
			})
	}

	lobby_id := LobbyId(in.LobbyId)
	lobbies[lobby_id] = lobby

	go func() {
		for {
			if lobbies[lobby_id].DayStage == DAY {
				for i, _ := range lobbies[lobby_id].Players {
					if lobbies[lobby_id].Players[i].IsRobot {
						continue
					}
					lobbies[lobby_id].Players[i].Notifications = append(
						lobbies[lobby_id].Players[i].Notifications,
						"Another day comes",
					)
				}

				for {
					if lobbies[lobby_id].IsReadyCount != lobbies[lobby_id].HumanNumber {
						time.Sleep(time.Second)
						continue
					}

					for i, player := range lobbies[lobby_id].Players {
						if !player.IsRobot 
					}
				}
			}
		}
	}()

	return &pb.StartGameReply{}, nil
}

func (server) ChooseAsMafia(ctx context.Context, in *pb.ChooseAsMafiaRequest) (*pb.ChooseAsMafiaReply, error) {
	player_ip := utils.ParseIp(ctx)
	log.Printf("Received start game %s", in.LobbyId)

	ip_info, ok := ip_infos[player_ip]
	if !ok || ip_info.lobby_id != LobbyId(in.LobbyId) {
		return nil, status.Error(400, "You are not connected to lobby")
	}

	lobby := lobbies[ip_info.lobby_id]
	if lobby.GameState != RUNNING {
		return nil, status.Error(400, "Game is not running")
	}

	if lobby.DayStage != NIGHT_MAFIA {
		return nil, status.Error(400, "It's not time to make mafia choose")
	}

	player := lobby.FindPlayer(ip_info.player_id)
	if player.Role != MAFIA {
		return nil, status.Error(400, "It can do only mafia")
	}

	if lobby.FindPlayer(PlayerId(in.ChoosedPlayer)) == nil {
		return nil, status.Error(400, "There is no selected player")
	}

	lobby.NightMafiaChoice = (*PlayerId)(&in.ChoosedPlayer)

	return &pb.ChooseAsMafiaReply{}, nil
}

func (server) ChooseAsMilitia(ctx context.Context, in *pb.ChooseAsMilitiaRequest) (*pb.ChooseAsMilitiaReply, error) {
	player_ip := utils.ParseIp(ctx)
	log.Printf("Received start game %s", in.LobbyId)

	ip_info, ok := ip_infos[player_ip]
	if !ok || ip_info.lobby_id != LobbyId(in.LobbyId) {
		return nil, status.Error(400, "You are not connected to lobby")
	}

	lobby := lobbies[ip_info.lobby_id]
	if lobby.GameState != RUNNING {
		return nil, status.Error(400, "Game is not running")
	}

	if lobby.DayStage != NIGHT_MILITIA {
		return nil, status.Error(400, "It's not time to make militia choose")
	}

	player := lobby.FindPlayer(ip_info.player_id)
	if player.Role != MILITIA {
		return nil, status.Error(400, "It can do only mafia")
	}

	if lobby.FindPlayer(PlayerId(in.ChoosedPlayer)) == nil {
		return nil, status.Error(400, "There is no selected player")
	}

	lobby.NightMafiaChoice = (*PlayerId)(&in.ChoosedPlayer)

	return &pb.ChooseAsMilitiaReply{}, nil
}

func (server) FinishDay(ctx context.Context, in *pb.FinishDayRequest) (*pb.FinishDayReply, error) {
	player_ip := utils.ParseIp(ctx)
	log.Printf("Received start game %s", in.LobbyId)

	ip_info, ok := ip_infos[player_ip]
	if !ok || ip_info.lobby_id != LobbyId(in.LobbyId) {
		return nil, status.Error(400, "You are not connected to lobby")
	}

	lobby := lobbies[ip_info.lobby_id]
	if lobby.GameState != RUNNING {
		return nil, status.Error(400, "Game is not running")
	}

	if lobby.DayStage != DAY {
		return nil, status.Error(400, "It's not day")
	}

	if _, ok := lobby.LynchVoting[ip_info.player_id]; !ok {
		return nil, status.Error(406, "You have to make lynch chooose first")
	}

	request_player := *lobby.FindPlayer(ip_info.player_id)

	lobby.IsReadyCount++
	for i, player := range lobby.Players {
		if player.Id == request_player.Id {
			continue
		}

		lobby.Players[i].Notifications = append(
			lobby.Players[i].Notifications,
			fmt.Sprintf("Player %s finished day", request_player.Name))
	}

	lobbies[ip_info.lobby_id] = lobby

	return &pb.FinishDayReply{}, nil
}

func (server) Lynch(ctx context.Context, in *pb.LynchRequest) (*pb.LynchReply, error) {
	player_ip := utils.ParseIp(ctx)
	log.Printf("Received start game %s", in.LobbyId)

	ip_info, ok := ip_infos[player_ip]
	if !ok || ip_info.lobby_id != LobbyId(in.LobbyId) {
		return nil, status.Error(400, "You are not connected to lobby")
	}

	lobby := lobbies[ip_info.lobby_id]
	if lobby.GameState != RUNNING {
		return nil, status.Error(400, "Game is not running")
	}

	if lobby.DayStage != DAY {
		return nil, status.Error(400, "It's not day")
	}

	if guilty := lobby.FindPlayer(PlayerId(in.GuiltyPlayerId)); guilty == nil {
		return nil, status.Error(400, "There is not such player")
	}

	request_player := *lobby.FindPlayer(ip_info.player_id)

	lobby.LynchVoting[ip_info.player_id] = PlayerId(in.GuiltyPlayerId)
	for i, player := range lobby.Players {
		if player.Id == request_player.Id {
			continue
		}

		lobby.Players[i].Notifications = append(
			lobby.Players[i].Notifications,
			fmt.Sprintf("Player %s selected %s to lynch", request_player.Name, in.GuiltyPlayerId))
	}

	lobbies[ip_info.lobby_id] = lobby

	return &pb.LynchReply{}, nil
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)

	}

	s := grpc.NewServer()
	pb.RegisterMafiaEngineServer(s, &server{})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
