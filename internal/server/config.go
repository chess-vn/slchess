package server

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

var (
	Port            string
	MatchingTimeout time.Duration
)

func init() {
	viper.SetConfigName("config") // name of config flie (no extension)
	viper.SetConfigType("json")
	viper.AddConfigPath(".infra/")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}

	Port = viper.GetString("host.game_server_port")

	MatchingTimeout = time.Duration(viper.GetInt("game.matching_timeout")) * time.Second
}
