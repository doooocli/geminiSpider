package main

import (
	"errors"
	"github.com/spf13/viper"
	"log"
	"os"
)

func init() {
	viper.SetConfigName("config")
	viper.SetConfigType("ini")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			log.Println("./config.ini file not found")
			os.Exit(1)
		}
	}
}
