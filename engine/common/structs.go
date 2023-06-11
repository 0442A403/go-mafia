package common

type Ip string
type LobbyId string
type PlayerId string

type PlayerRole uint8

const (
	NOT_SETTED PlayerRole = iota
	MAFIA
	MILITIA
	SEEKER
)

func (role PlayerRole) Stringify() string {
	switch role {
	case MAFIA:
		return "mafia"
	case MILITIA:
		return "militia"
	case SEEKER:
		return "seeker"
	default:
		return "unsetted"
	}
}

type GameState uint8

const (
	NOT_STARTED GameState = iota
	RUNNING
	FINISHED
)

type DayStage uint8

const (
	DAY DayStage = iota
	NIGHT_MILITIA
	NIGHT_MAFIA
)

type Player struct {
	Id            PlayerId
	Ip            Ip
	Name          string
	LobbyId       LobbyId
	IsDead        bool
	Role          PlayerRole
	Notifications []string
	IsRobot       bool
	IsReady       bool
}

type Lobby struct {
	LobbyId            LobbyId
	CreatorId          PlayerId
	Players            []Player
	GameState          GameState
	DayStage           DayStage
	DayNumber          int
	NightMafiaChoice   *PlayerId
	NightMilitiaChoice *PlayerId
	LynchVoting        map[PlayerId]PlayerId
}

func (l Lobby) FindPlayer(player_id PlayerId) *Player {
	for _, player := range l.Players {
		if player.Id == player_id {
			return &player
		}
	}
	return nil
}
