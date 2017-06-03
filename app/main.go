package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gocraft/web"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

type App struct {
}

var (
	myLogger = logging.MustGetLogger("app")

	// chaincode url
	adaptorURL string
)

func buildRouter() *web.Router {
	router := web.New(App{})

	// Add middleware
	router.Middleware((*App).SetResponseType)

	api := router.Subrouter(App{}, "/api")
	api.Post("/login", (*App).Login)
	api.Post("/logout", (*App).Logout)
	api.Get("/islogin", (*App).IsLogin)
	api.Get("/mycurrency", (*App).MyCurrency)
	api.Get("/myasset", (*App).MyAsset)
	api.Get("/currencys", (*App).Currencys)
	api.Get("/history", (*App).History)
	api.Get("/users", (*App).Users)

	// Add routes
	currencyRouter := api.Subrouter(App{}, "/currency")
	currencyRouter.Get("/:id", (*App).Currency)
	currencyRouter.Post("/create", (*App).Create)
	currencyRouter.Post("/release", (*App).Release)
	currencyRouter.Post("/assign", (*App).Assign)
	currencyRouter.Get("/create/:txid", (*App).CheckCreate)
	currencyRouter.Get("/release/:txid", (*App).CheckRelease)
	currencyRouter.Get("/assign/:txid", (*App).CheckAssign)

	txRouter := api.Subrouter(App{}, "/tx")
	txRouter.Get("/:uuid", (*App).Tx)
	txRouter.Get("/market/:srccurrency/:descurrency/:count", (*App).Market)
	txRouter.Post("/exchange", (*App).Exchange)
	txRouter.Post("/cancel", (*App).Cancel)
	txRouter.Get("/exchange/:uuid", (*App).CheckOrder)
	txRouter.Get("/cancel/:uuid", (*App).CheckCancel)
	txRouter.Get("/my/:status/:count", (*App).MyTxs)
	txRouter.Get("/:srccurrency/:descurrency/:count", (*App).CurrencysTxs)

	// Add not found page
	router.NotFound((*App).NotFound)

	return router
}

func initConfig() {
	// Now set the configuration file
	viper.SetEnvPrefix("APP")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
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

	adaptorURL = viper.GetString("app.adaptor.address")

	// Deploy
	if err := deployBase(); err != nil {
		myLogger.Errorf("Failed deploying base chaincode [%s]", err)
		os.Exit(-1)
	}

	if err := deployBus(); err != nil {
		myLogger.Errorf("Failed deploying business chaincode [%s]", err)
		os.Exit(-1)
	}

	time.Sleep(2 * time.Minute)

	if _, err := initTable(); err != nil {
		myLogger.Errorf("Failed init table for business chaincode [%s]", err)
		os.Exit(-1)
	}

	// 保存chaincodeID 供监听chaincode事件使用
	chaincodeKey := viper.GetString("app.event.chaincode.key")
	if err := setString(chaincodeKey, chaincodeNameBus); err != nil {
		myLogger.Errorf("Failed save chaincodeID [%s]", err)
		os.Exit(-1)
	}

	go task()

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
