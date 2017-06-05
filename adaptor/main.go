package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gocraft/web"
	"github.com/hyperledger/fabric/core/crypto"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

var (
	myLogger = logging.MustGetLogger("adaptor")
)

func initConfig() {
	viper.SetEnvPrefix("ADAPTOR")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s", err))
	}
}

func buildRouter() *web.Router {
	router := web.New(Adaptor{})

	router.Middleware((*Adaptor).SetResponseType)
	router.NotFound((*Adaptor).NotFound)

	router.Post("/login", (*Adaptor).Login)
	router.Post("/deploy", (*Adaptor).Deploy)
	router.Post("/invoke", (*Adaptor).Invoke)
	router.Post("/query", (*Adaptor).Query)

	return router
}

func main() {
	initConfig()

	crypto.Init()

	// 链码间调用时，保密级别必须是 pb.ConfidentialityLevel_PUBLIC /fabric/core/chaincode/handler.go 179
	confidentiality(viper.GetBool("security.privacy"))

	if err := initPeerClient(); err != nil {
		myLogger.Errorf("Failed initiliazing PeerClient [%s]", err)
		os.Exit(-1)
	}

	restAddress := viper.GetString("adaptor.rest.address")
	tlsEnable := viper.GetBool("adaptor.tls.enable")

	myLogger.Infof("Initializing the REST service on %s, TLS is %s.", restAddress, (map[bool]string{true: "enable", false: "disenable"})[tlsEnable])

	router := buildRouter()

	if tlsEnable {
		err := http.ListenAndServeTLS(restAddress, viper.GetString("adaptor.tls.cert.file"), viper.GetString("adaptor.tls.key.file"), router)
		if err != nil {
			myLogger.Errorf("ListenAndServeTLS error : %s", err)
		}
	} else {
		err := http.ListenAndServe(restAddress, router)
		if err != nil {
			myLogger.Errorf("ListenAndServe error : %s", err)
		}
	}
}
