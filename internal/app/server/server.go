package server

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/chess-vn/slchess/internal/domains/entities"
	"github.com/chess-vn/slchess/pkg/logging"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type server struct {
	address  string
	upgrader websocket.Upgrader

	config  Config
	matches sync.Map

	cognitoPublicKeys map[string]*rsa.PublicKey
	shutdownTimer     *time.Timer
}

type payload struct {
	Type      string            `json:"type"`
	Data      map[string]string `json:"data"`
	CreatedAt time.Time         `json:"created_at"`
}

func NewServer() *server {
	config := NewConfig()
	srv := &server{
		address: "0.0.0.0:" + config.Port,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins
			},
		},
		config:            config,
		cognitoPublicKeys: make(map[string]*rsa.PublicKey),
	}
	srv.loadCognitoPublicKeys()
	srv.resetShutdownTimer()
	return srv
}

// Start method    starts the game server
func (s *server) Start() error {
	http.HandleFunc("/game/{matchId}", func(w http.ResponseWriter, r *http.Request) {
		playerId, err := s.auth(r)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(err.Error()))
			return
		}

		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			logging.Error("failed to upgrade connection", zap.String("error", err.Error()))
			return
		}
		defer conn.Close()

		s.resetShutdownTimer()

		matchId := r.PathValue("matchId")
		match, err := s.loadMatch(matchId)
		if err != nil {
			logging.Error("failed to load match", zap.String("error", err.Error()))
			return
		}
		s.handlePlayerJoin(conn, match, playerId)

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseMessage, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logging.Info("unexpected close error", zap.String("remote_address", conn.RemoteAddr().String()))
				} else if websocket.IsCloseError(err, websocket.CloseMessage, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logging.Info("connection closed", zap.String("remote_address", conn.RemoteAddr().String()))
				} else {
					logging.Info("ws message read error", zap.String("remote_address", conn.RemoteAddr().String()), zap.Error(err))
				}
				s.handlePlayerDisconnect(match, playerId)
				break
			}

			payload := payload{}
			if err := json.Unmarshal(message, &payload); err != nil {
				conn.Close()
			}
			s.handleWebSocketMessage(playerId, match, payload)
		}
	})
	logging.Info("websocket server started", zap.String("port", s.config.Port))
	return http.ListenAndServe(s.address, nil)
}

// mustAuth method    authenticates and extract userId
func (s *server) auth(r *http.Request) (string, error) {
	token := r.Header.Get("Authorization")
	if token == "" {
		return "", fmt.Errorf("no authorization")
	}
	validToken, err := s.validateJWT(token)
	if err != nil || !validToken.Valid {
		return "", fmt.Errorf("invalid token: %w", err)
	}
	mapClaims, ok := validToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid map claims")
	}
	v, ok := mapClaims["sub"]
	if !ok {
		return "", fmt.Errorf("user id not found")
	}
	userId, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("invalid user id")
	}
	return userId, nil
}

// loadMatch method    loads match with corresponding matchId.
// If no such match exists, create a new match.
// This is used to start the match only when white side player send in the first valid move.
func (s *server) loadMatch(matchId string) (*Match, error) {
	ctx := context.Background()
	cfg, _ := config.LoadDefaultConfig(ctx)
	dynamoClient := dynamodb.NewFromConfig(cfg)
	activeMatchOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("ActiveMatches"),
		Key: map[string]types.AttributeValue{
			"MatchId": &types.AttributeValueMemberS{
				Value: matchId,
			},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	if activeMatchOutput.Item == nil {
		return nil, fmt.Errorf("match not found: %s", matchId)
	}
	var activeMatch entities.ActiveMatch
	attributevalue.UnmarshalMap(activeMatchOutput.Item, &activeMatch)

	value, loaded := s.matches.Load(matchId)
	if loaded {
		match, ok := value.(*Match)
		if ok {
			logging.Info("match loaded")
			return match, nil
		}
		return nil, ErrFailedToLoadMatch
	} else {
		matchStateOutput, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String("MatchStates"),
			Key: map[string]types.AttributeValue{
				"MatchId": &types.AttributeValueMemberS{
					Value: matchId,
				},
			},
			ConsistentRead: aws.Bool(true),
		})
		if err != nil {
			return nil, err
		}

		var matchState entities.MatchState
		attributevalue.UnmarshalMap(matchStateOutput.Item, &matchState)

		config := configForGameMode(activeMatch.GameMode)
		var clock1 time.Duration
		var clock2 time.Duration

		// Initialize match if there is no match state data
		if matchStateOutput.Item == nil {
			clock1 = config.MatchDuration
			clock2 = config.MatchDuration
			logging.Info("match initialized")
		} else {
			clock1, _ = time.ParseDuration(matchState.Players[0].Clock)
			clock2, _ = time.ParseDuration(matchState.Players[1].Clock)
			logging.Info("match resumed")
		}
		player1 := newPlayer(
			nil,
			activeMatch.Player1.Id,
			WHITE_SIDE,
			clock1,
			activeMatch.Player1.Rating,
			activeMatch.Player1.NewRatings,
			activeMatch.Player1.NewRDs,
		)
		player2 := newPlayer(
			nil,
			activeMatch.Player2.Id,
			BLACK_SIDE,
			clock2,
			activeMatch.Player2.Rating,
			activeMatch.Player2.NewRatings,
			activeMatch.Player1.NewRDs,
		)
		match := s.newMatch(matchId, player1, player2, config)
		s.matches.Store(matchId, match)
		return match, nil
	}
}

func (s *server) newMatch(matchId string, player1, player2 player, config MatchConfig) *Match {
	match := &Match{
		id:              matchId,
		game:            newGame(),
		players:         []*player{&player1, &player2},
		moveCh:          make(chan move),
		config:          config,
		endGameHandler:  s.handleEndGame,
		saveGameHandler: s.handleSaveGame,
	}
	// Timeout to cancel match if first move is not made
	match.setTimer(config.CancelTimeout)
	go match.start()
	return match
}

func (s *server) removeMatch(matchId string) {
	s.matches.Delete(matchId)
}

func (s *server) resetShutdownTimer() {
	if s.shutdownTimer != nil {
		s.shutdownTimer.Reset(s.config.IdleTimeout)
		return
	}
	s.shutdownTimer = time.NewTimer(s.config.IdleTimeout)
	go func() {
		<-s.shutdownTimer.C
		s.shutdown()
	}()
	logging.Info("shutdowm timer set", zap.String("duration", s.config.IdleTimeout.String()))
}

func (s *server) shutdown() {
	logging.Info("server terminating")
	s.matches.Range(func(key, value interface{}) bool {
		match, ok := value.(*Match)
		if ok {
			match.end()
		}
		return true
	})
	os.Exit(0)
}
