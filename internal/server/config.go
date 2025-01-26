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

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs/server")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}

	config.Port = viper.GetString("server.port")

	return config
}
