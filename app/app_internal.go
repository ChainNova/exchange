package main

import (
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric/core/crypto"
	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/protos"
	"github.com/spf13/viper"
)

// Chaincode Chaincode
type Chaincode struct {
	ID      *pb.ChaincodeID       `json:"id"`
	Type    pb.ChaincodeSpec_Type `json:"type"`
	Input   *pb.ChaincodeInput    `json:"input"`
	User    pb.Secret             `json:"user"`
	invoker crypto.Client
}

var (
	chaincodePath string
	chaincodeName string
	chaincodeType pb.ChaincodeSpec_Type
	admin         string
)

func deploy() (err error) {
	chaincodePath = viper.GetString("chaincode.id.path")
	chaincodeName = viper.GetString("chaincode.id.name")
	ccType := viper.GetString("chaincode.type")
	admin = viper.GetString("app.admin.name")

	if ccType == "golang" {
		chaincodeType = pb.ChaincodeSpec_GOLANG
	} else if ccType == "java" {
		chaincodeType = pb.ChaincodeSpec_JAVA
	} else {
		return fmt.Errorf("Unknow chiancode type: %s", ccType)
	}

	if chaincodeName != "" {
		myLogger.Infof("Using existing chaincode [%s]", chaincodeName)
		return
	}

	err = login(&pb.Secret{
		EnrollId:     admin,
		EnrollSecret: viper.GetString("app.admin.pwd"),
	})
	if err != nil {
		myLogger.Errorf("Failed login [%s]", err)
		return
	}

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Path: chaincodePath},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("init")},
		User:  pb.Secret{EnrollId: admin},
	}

	return deployChaincode(&chaincode)
}

func createCurrency(currency string, count int64, user string) (txid string, err error) {
	myLogger.Debugf("Chaincode [create] args:[%s]-[%s],[%s]-[%s]", "currency", currency, "count", count)

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Name: chaincodeName},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("create", currency, strconv.FormatInt(count, 10), user)},
		User:  pb.Secret{EnrollId: user},
	}

	return invokeChaincode(&chaincode)
}

func releaseCurrency(currency string, count int64, user string) (txid string, err error) {
	myLogger.Debugf("Chaincode [release] args:[%s]-[%s],[%s]-[%s]", "currency", currency, "count", count)

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Name: chaincodeName},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("release", currency, strconv.FormatInt(count, 10))},
		User:  pb.Secret{EnrollId: user},
	}

	return invokeChaincode(&chaincode)
}

func assignCurrency(assigns string, user string) (txid string, err error) {
	myLogger.Debugf("Chaincode [assign] args:[%s]-[%s]", "assigns", assigns)

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Name: chaincodeName},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("assign", assigns)},
		User:  pb.Secret{EnrollId: user},
	}

	return invokeChaincode(&chaincode)
}

func exchange(exchanges string) (err error) {
	myLogger.Debugf("Chaincode [exchange] args:[%s]-[%s]", "exchanges", exchanges)

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Name: chaincodeName},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("exchange", exchanges)},
		User:  pb.Secret{EnrollId: admin},
	}

	_, err = invokeChaincode(&chaincode)
	return
}

func lock(orders string, islock bool, srcMethod string) (txid string, err error) {
	myLogger.Debugf("Chaincode [lock] args:[%s]-[%s],[%s]-[%s],[%s]-[%s]", "orders", orders, "islock", islock, "srcMethod", srcMethod)

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Name: chaincodeName},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("lock", orders, strconv.FormatBool(islock), srcMethod)},
		User:  pb.Secret{EnrollId: admin},
	}

	return invokeChaincode(&chaincode)
}

func getCurrencys() (currencys string, err error) {
	myLogger.Debug("Chaincode [queryAllCurrency] args:[]")

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Name: chaincodeName},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryAllCurrency")},
		User:  pb.Secret{EnrollId: admin},
	}

	return queryChaincode(&chaincode)
}

func getCurrency(id string) (currency string, err error) {
	myLogger.Debugf("Chaincode [queryCurrencyByID] args:[%s]-[%s]", "id", id)

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Name: chaincodeName},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryCurrencyByID", id)},
		User:  pb.Secret{EnrollId: admin},
	}

	return queryChaincode(&chaincode)
}

func getCurrencysByUser(user string) (currencys string, err error) {
	myLogger.Debugf("Chaincode [getCurrencysByUser] args:[%s]-[%s]", "user", user)

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Name: chaincodeName},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryMyCurrency", user)},
		User:  pb.Secret{EnrollId: user},
	}

	return queryChaincode(&chaincode)
}

func getAsset(user string) (asset string, err error) {
	myLogger.Debugf("Chaincode [queryAssetByOwner] args:[%s]-[%s]", "owner", user)

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Name: chaincodeName},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryAssetByOwner", user)},
		User:  pb.Secret{EnrollId: user},
	}

	return queryChaincode(&chaincode)
}

func getTxLogs() (txLogs string, err error) {
	myLogger.Debug("Chaincode [queryTxLogs] args:[]")

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Name: chaincodeName},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryTxLogs")},
		User:  pb.Secret{EnrollId: admin},
	}

	return queryChaincode(&chaincode)
}

func initAccount(user string) (result string, err error) {
	myLogger.Debugf("Chaincode [initAccount] args:[%s]-[%s]", "initAccount", user)

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Name: chaincodeName},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("initAccount", user)},
		User:  pb.Secret{EnrollId: user},
	}

	_, err = invokeChaincode(&chaincode)
	return
}

func getMyReleaseLog(user string) (log string, err error) {
	myLogger.Debugf("Chaincode [getMyReleaseLog] args:[%s]-[%s]", "user", user)

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Name: chaincodeName},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryMyReleaseLog", user)},
		User:  pb.Secret{EnrollId: user},
	}

	return queryChaincode(&chaincode)
}

func getMyAssignLog(user string) (log string, err error) {
	myLogger.Debugf("Chaincode [getMyAssignLog] args:[%s]-[%s]", "user", user)

	chaincode := Chaincode{
		ID:    &pb.ChaincodeID{Name: chaincodeName},
		Type:  chaincodeType,
		Input: &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryMyAssignLog", user)},
		User:  pb.Secret{EnrollId: user},
	}

	return queryChaincode(&chaincode)
}
