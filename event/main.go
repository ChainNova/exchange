package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hyperledger/fabric/events/consumer"
	pb "github.com/hyperledger/fabric/protos"
	logging "github.com/op/go-logging"
	"github.com/spf13/viper"
)

var (
	myLogger    = logging.MustGetLogger("event")
	a           *adapter
	chaincodeID string
	obcEHClient *consumer.EventsClient
)

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

	initRedis()
	defer client.Close()

	go checkChaincodeID()

	chaincodeID, _ = getChaincodeID()
	if chaincodeID == "" {
		myLogger.Error("Can't find chaincode!!!")
	}

	a = &adapter{
		blockEvent:     make(chan *pb.Event_Block),
		chaincodeEvent: make(chan *pb.Event_ChaincodeEvent),
		rejectionEvent: make(chan *pb.Event_Rejection),
		chaincodeID:    chaincodeID}

	eventListener(a)

	for {
		select {
		case b := <-a.blockEvent:
			myLogger.Debug("Received block\n")
			myLogger.Debug("--------------\n")
			for _, r := range b.Block.Transactions {
				myLogger.Debugf("Transaction:\n\t[%v]\n", r)
				setChaincodeResult(r.Txid, Chaincode_Success)
			}
		case r := <-a.rejectionEvent:
			myLogger.Debug("Received rejected transaction\n")
			myLogger.Debug("--------------\n")
			myLogger.Debugf("Transaction error:\n%s\n", r.Rejection.ErrorMsg)

			if r.Rejection.Tx != nil {
				setChaincodeResult(r.Rejection.Tx.Txid, r.Rejection.ErrorMsg)
			}
		case ce := <-a.chaincodeEvent:
			myLogger.Debug("Received chaincode event\n")
			myLogger.Debug("------------------------\n")
			myLogger.Debugf("Chaincode Event:%v\n", ce)

			var batch BatchResult
			err := json.Unmarshal(ce.ChaincodeEvent.Payload, &batch)
			if err != nil {
				continue
			}
			setChaincodeBatchResult(ce.ChaincodeEvent.TxID, batch)
		}
	}
}
