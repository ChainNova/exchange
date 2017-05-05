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

type AppREST struct {
}

var (
	myLogger = logging.MustGetLogger("app")

	// chaincode url
	restURL  string
	connPeer string
	admin    string
)

func buildRouter() *web.Router {
	router := web.New(AppREST{})

	// Add middleware
	router.Middleware((*AppREST).SetResponseType)

	api := router.Subrouter(AppREST{}, "/api")
	api.Post("/login", (*AppREST).Login)
	api.Post("/logout", (*AppREST).Logout)
	api.Get("/islogin", (*AppREST).IsLogin)
	api.Get("/mycurrency", (*AppREST).MyCurrency)
	api.Get("/myasset", (*AppREST).MyAsset)
	api.Get("/currencys", (*AppREST).Currencys)
	api.Get("/history", (*AppREST).History)
	api.Get("/users", (*AppREST).Users)

	// Add routes
	currencyRouter := api.Subrouter(AppREST{}, "/currency")
	currencyRouter.Get("/:id", (*AppREST).Currency)
	currencyRouter.Post("/create", (*AppREST).Create)
	currencyRouter.Post("/release", (*AppREST).Release)
	currencyRouter.Post("/assign", (*AppREST).Assign)
	currencyRouter.Get("/create/:txid", (*AppREST).CheckCreate)
	currencyRouter.Get("/release/:txid", (*AppREST).CheckRelease)
	currencyRouter.Get("/assign/:txid", (*AppREST).CheckAssign)

	txRouter := api.Subrouter(AppREST{}, "/tx")
	txRouter.Get("/:uuid", (*AppREST).Tx)
	txRouter.Get("/market/:srccurrency/:descurrency/:count", (*AppREST).Market)
	txRouter.Post("/exchange", (*AppREST).Exchange)
	txRouter.Post("/cancel", (*AppREST).Cancel)
	txRouter.Get("/exchange/:uuid", (*AppREST).CheckOrder)
	txRouter.Get("/cancel/:uuid", (*AppREST).CheckCancel)
	txRouter.Get("/my/:status/:count", (*AppREST).MyTxs)
	txRouter.Get("/:srccurrency/:descurrency/:count", (*AppREST).CurrencysTxs)

	// Add not found page
	router.NotFound((*AppREST).NotFound)

	return router
}

func initConfig() {
	// Now set the configuration file
	viper.SetEnvPrefix("HYPERLEDGER")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.SetConfigName("config") // name of config file (without extension)
	viper.AddConfigPath(".")      // path to look for the config file in
	err := viper.ReadInConfig()   // Find and read the config file
	if err != nil {               // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
}

func main() {
	initConfig()

	initRedis()
	defer client.Close()

	crypto.Init()
	// Enable fabric 'confidentiality'
	confidentiality(viper.GetBool("security.privacy"))
	admin = viper.GetString("app.admin.name")
	connPeer = viper.GetString("app.connpeer")
	myLogger.Debugf("The peer connection type is: %s ", connPeer)

	if connPeer == "grpc" {
		if err := initNVP(); err != nil {
			myLogger.Errorf("Failed initiliazing NVP [%s]", err)
			os.Exit(-1)
		}
	} else if connPeer == "rest" {
		restURL = viper.GetString("rest.address")
	} else {
		myLogger.Errorf("connPeer not know")
		os.Exit(-1)
	}

	// Deploy
	if err := deploy(); err != nil {
		myLogger.Errorf("Failed deploying [%s]", err)
		os.Exit(-1)
	}

	go eventListener(chaincodeName)

	go lockBalance()

	go matchTx()

	go execTx()

	go findExpired()

	go execExpired()

	go execCancel()

	restAddress := viper.GetString("app.rest.address")
	tlsEnable := viper.GetBool("app.tls.enabled")

	// Initialize the REST service object
	myLogger.Infof("Initializing the REST service on %s, TLS is %s.", restAddress, (map[bool]string{true: "enabled", false: "disabled"})[tlsEnable])

	router := buildRouter()

	// Start server
	if tlsEnable {
		err := http.ListenAndServeTLS(restAddress, viper.GetString("app.tls.cert.file"), viper.GetString("app.tls.key.file"), router)
		if err != nil {
			myLogger.Errorf("ListenAndServeTLS: %s", err)
		}
	} else {
		err := http.ListenAndServe(restAddress, router)
		if err != nil {
			myLogger.Errorf("ListenAndServe: %s", err)
		}
	}
}
