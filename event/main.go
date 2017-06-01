package main

import (
	"fmt"
	"strings"

	logging "github.com/op/go-logging"
	"github.com/spf13/viper"
)

var myLogger = logging.MustGetLogger("event")

func initConfig() {
	// Now set the configuration file
	viper.SetEnvPrefix("EVENT")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetConfigName("config") // name of config file (without extension)
	viper.AddConfigPath(".")      // path to look for the config file in
	err := viper.ReadInConfig()   // Find and read the config file
	if err != nil {               // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s", err))
	}
}

func main() {
	initConfig()

	go eventListener()

	go checkChaincodeID()
}
