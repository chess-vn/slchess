package server

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Port string
}

func NewConfig() Config {
	var config Config

	viper.SetConfigName("config") // name of config flie (no extension)
	viper.SetConfigType("json")
	viper.AddConfigPath(".infra/")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}

	config.Port = viper.GetString("SERVER_PORT")

	return config
}
