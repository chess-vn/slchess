package stofinet

import (
	"fmt"
	"net/url"

	"github.com/chess-vn/slchess/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Config struct {
	StockfishPath string
	BaseUrl       *url.URL
}

func LoadConfig() (Config, error) {
	var cfg Config

	// List of env files to load
	envFiles := []string{
		"./configs/stofinet/app.env",
	}

	// Load into config struct
	err := loadEnvFiles(envFiles)
	if err != nil {
		logging.Fatal("fatal error config file", zap.Error(err))
	}

	cfg.BaseUrl, err = url.Parse(viper.GetString("BASE_URL"))
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse base url: %w", err)
	}
	cfg.StockfishPath = viper.GetString("STOCKFISH_PATH")

	return cfg, nil
}

func loadEnvFiles(filenames []string) error {
	for _, file := range filenames {
		viper.SetConfigFile(file)
		viper.SetConfigType("env")
		viper.AutomaticEnv()

		err := viper.MergeInConfig()
		if err != nil {
			return err
		}
	}
	return nil
}
