package server

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Port        string
	IdleTimeout time.Duration

	AwsRegion            string
	CognitoUserPoolId    string
	AppSyncHttpUrl       string
	AppSyncAccessRoleArn string

	EndGameFunctionArn string
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

	// List of env files to load
	envFiles := []string{
		"./configs/aws/base.env",
		"./configs/aws/cognito.env",
		"./configs/aws/lambda.env",
		"./configs/aws/appsync.env",
	}

	// Load all env files
	err = loadEnvFiles(envFiles)
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}

	config.Port = viper.GetString("Server.Port")
	idleTimeout, err := time.ParseDuration(viper.GetString("Server.IdleTimeout"))
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}
	config.IdleTimeout = idleTimeout
	config.AwsRegion = viper.GetString("AWS_REGION")
	config.CognitoUserPoolId = viper.GetString("COGNITO_USER_POOL_ID")
	config.AppSyncHttpUrl = viper.GetString("APPSYNC_HTTP_URL")
	config.AppSyncAccessRoleArn = viper.GetString("APPSYNC_ACCESS_ROLE_ARN")
	config.EndGameFunctionArn = viper.GetString("END_GAME_FUNCTION_ARN")

	return config
}

func loadEnvFiles(filenames []string) error {
	for _, file := range filenames {
		viper.SetConfigFile(file) // Set specific file
		viper.SetConfigType("env")
		viper.AutomaticEnv() // Allow override by OS environment variables

		err := viper.MergeInConfig()
		if err != nil {
			return err
		}
	}
	return nil
}
