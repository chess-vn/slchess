package server

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/chess-vn/slchess/internal/aws/auth"
	awsAuth "github.com/chess-vn/slchess/internal/aws/auth"
	"github.com/chess-vn/slchess/internal/aws/storage"
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
	mu      sync.Mutex

	cognitoPublicKeys map[string]*rsa.PublicKey
	storageClient     *storage.Client
}

type payload struct {
	Type string            `json:"type"`
	Data map[string]string `json:"data"`
}

func NewServer() *server {
	cfg := NewConfig()
	tokenSigningKeyUrl := fmt.Sprintf(
		"https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json",
		cfg.AwsRegion,
		cfg.CognitoUserPoolId,
	)
	cognitoPublicKeys, err := awsAuth.LoadCognitoPublicKeys(tokenSigningKeyUrl)
	if err != nil {
		panic(err)
	}
	awsCfg, _ := config.LoadDefaultConfig(context.TODO())
	srv := &server{
		address: "0.0.0.0:" + cfg.Port,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins
			},
		},
		config:            cfg,
		cognitoPublicKeys: cognitoPublicKeys,
		storageClient: storage.NewClient(
			dynamodb.NewFromConfig(awsCfg),
		),
	}
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
			logging.Error(
				"failed to upgrade connection",
				zap.String("error", err.Error()),
			)
			return
		}
		defer conn.Close()

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
				s.handlePlayerDisconnect(match, playerId)
				logging.Info(
					"connection closed",
					zap.String("remote_address", conn.RemoteAddr().String()),
					zap.Error(err),
				)
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
	validToken, err := auth.ValidateJwt(token, s.cognitoPublicKeys)
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

/*
loadMatch method    loads match with corresponding matchId.
If no such match exists, create a new match.
This is used to start the match only when white side player send in the first valid move.
*/
func (s *server) loadMatch(matchId string) (*Match, error) {
	ctx := context.Background()

	activeMatch, err := s.storageClient.GetActiveMatch(ctx, matchId)
	if err != nil {
		return nil, fmt.Errorf("failed to get active match: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	value, loaded := s.matches.Load(matchId)
	if loaded {
		match, ok := value.(*Match)
		if ok {
			logging.Info("match loaded")
			return match, nil
		}
		return nil, ErrFailedToLoadMatch
	} else {
		matchStates, _, err := s.storageClient.FetchMatchStates(
			ctx,
			matchId,
			nil,
			1,
			false,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch match states: %w", err)
		}

		config, err := configForGameMode(activeMatch.GameMode)
		if err != nil {
			return nil, err
		}
		var clock1 time.Duration
		var clock2 time.Duration

		// Initialize match if there is no match state data
		if len(matchStates) == 0 {
			clock1 = config.MatchDuration
			clock2 = config.MatchDuration
			logging.Info("match initialized")
		} else {
			clock1, _ = time.ParseDuration(matchStates[0].PlayerStates[0].Clock)
			clock2, _ = time.ParseDuration(matchStates[0].PlayerStates[1].Clock)
			logging.Info("match resumed")
		}
		player1 := newPlayer(
			nil,
			activeMatch.Player1.Id,
			WHITE_SIDE,
			clock1,
			activeMatch.Player1.Rating,
			activeMatch.Player1.RD,
			activeMatch.Player1.NewRatings,
			activeMatch.Player1.NewRDs,
		)
		player2 := newPlayer(
			nil,
			activeMatch.Player2.Id,
			BLACK_SIDE,
			clock2,
			activeMatch.Player2.Rating,
			activeMatch.Player2.RD,
			activeMatch.Player2.NewRatings,
			activeMatch.Player1.NewRDs,
		)
		match := s.newMatch(matchId, player1, player2, config)
		s.matches.Store(matchId, match)
		return match, nil
	}
}

func (s *server) newMatch(
	matchId string,
	player1,
	player2 player,
	config MatchConfig,
) *Match {
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
