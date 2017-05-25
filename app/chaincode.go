package main

import (
	"fmt"
	"strconv"

	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/protos"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var (
	peerClientConn *grpc.ClientConn
	serverClient   pb.PeerClient
	chaincodePath  string
	chaincodeName  string
	chaincodeType  pb.ChaincodeSpec_Type
)

func deploy() (err error) {
	chaincodePath = viper.GetString("chaincode.id.path")
	chaincodeName = viper.GetString("chaincode.id.name")
	ccType := viper.GetString("chaincode.id.type")

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

	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("init")}

	if connPeer == "grpc" {
		return deployChaincodeGrpc(chaincodeInput)
	}
	return deployChaincodeRest(chaincodeInput)
}

func createCurrency(currency string, count int64, user string) (txid string, err error) {
	myLogger.Debugf("Chaincode [create] args:[%s]-[%s],[%s]-[%s]", "currency", currency, "count", count)

	// chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("createCurrency", currency, strconv.FormatInt(count, 10), base64.StdEncoding.EncodeToString(invokerCert.GetCertificate()))}
	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("create", currency, strconv.FormatInt(count, 10), user)}

	if connPeer == "grpc" {
		invoker, err := setCryptoClient(user, "")
		if err != nil {
			myLogger.Errorf("Failed getting invoker [%s]", err)
			return "", err
		}
		// invokerCert, err := invoker.GetTCertificateHandlerNext()
		// if err != nil {
		// 	myLogger.Errorf("Failed getting TCert [%s]", err)
		// 	return
		// }

		return invokeChaincodeGrpc(user, invoker, chaincodeInput)
	}
	return invokeChaincodeRest(user, chaincodeInput)

}

func releaseCurrency(currency string, count int64, user string) (txid string, err error) {

	myLogger.Debugf("Chaincode [release] args:[%s]-[%s],[%s]-[%s]", "currency", currency, "count", count)

	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("release", currency, strconv.FormatInt(count, 10))}

	// return invokeChaincodeSigma(invoker, invokerCert, chaincodeInput)
	if connPeer == "grpc" {
		invoker, err := setCryptoClient(user, "")
		if err != nil {
			myLogger.Errorf("Failed getting invoker [%s]", err)
			return "", err
		}
		// invokerCert, err := invoker.GetTCertificateHandlerNext()
		// if err != nil {
		// 	myLogger.Errorf("Failed getting TCert [%s]", err)
		// 	return
		// }
		return invokeChaincodeGrpc(user, invoker, chaincodeInput)
	}
	return invokeChaincodeRest(user, chaincodeInput)

}

func assignCurrency(assigns string, user string) (txid string, err error) {

	myLogger.Debugf("Chaincode [assign] args:[%s]-[%s]", "assigns", assigns)

	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("assign", assigns)}

	// return invokeChaincodeSigma(invoker, invokerCert, chaincodeInput)
	if connPeer == "grpc" {
		invoker, err := setCryptoClient(user, "")
		if err != nil {
			myLogger.Errorf("Failed getting invoker [%s]", err)
			return "", err
		}
		// invokerCert, err := invoker.GetTCertificateHandlerNext()
		// if err != nil {
		// 	myLogger.Errorf("Failed getting TCert [%s]", err)
		// 	return
		// }
		return invokeChaincodeGrpc(user, invoker, chaincodeInput)
	}
	return invokeChaincodeRest(user, chaincodeInput)

}

func exchange(exchanges string) (err error) {
	myLogger.Debugf("Chaincode [exchange] args:[%s]-[%s]", "exchanges", exchanges)

	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("exchange", exchanges)}

	if connPeer == "grpc" {
		_, err = invokeChaincodeGrpc(admin, adminInvoker, chaincodeInput)
		return
	}
	_, err = invokeChaincodeRest(admin, chaincodeInput)
	return
}

func lock(orders string, islock bool, srcMethod string) (txid string, err error) {
	myLogger.Debugf("Chaincode [lock] args:[%s]-[%s],[%s]-[%s],[%s]-[%s]", "orders", orders, "islock", islock, "srcMethod", srcMethod)

	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("lock", orders, strconv.FormatBool(islock), srcMethod)}

	if connPeer == "grpc" {
		return invokeChaincodeGrpc(admin, adminInvoker, chaincodeInput)
	}
	return invokeChaincodeRest(admin, chaincodeInput)

}

func getCurrencys() (currencys string, err error) {
	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryAllCurrency")}

	if connPeer == "grpc" {
		return queryChaincodeGrpc(admin, chaincodeInput)
	}
	return queryChaincodeRest(admin, chaincodeInput)

}

func getCurrency(id string) (currency string, err error) {
	myLogger.Debugf("Chaincode [queryCurrencyByID] args:[%s]-[%s]", "id", id)

	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryCurrencyByID", id)}

	if connPeer == "grpc" {
		return queryChaincodeGrpc(admin, chaincodeInput)
	}
	return queryChaincodeRest(admin, chaincodeInput)

}

func getCurrencysByUser(user string) (currencys string, err error) {
	// invoker, err := setCryptoClient(user, "")
	// if err != nil {
	// 	myLogger.Errorf("Failed getting invoker [%s]", err)
	// 	return
	// }
	// invokerCert, err := invoker.GetTCertificateHandlerNext()
	// if err != nil {
	// 	myLogger.Errorf("Failed getting TCert [%s]", err)
	// 	return
	// }

	// cert := base64.StdEncoding.EncodeToString(invokerCert.GetCertificate())
	myLogger.Debugf("Chaincode [getCurrencysByUser] args:[%s]-[%s]", "user", user)

	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryMyCurrency", user)}

	if connPeer == "grpc" {
		return queryChaincodeGrpc(user, chaincodeInput)
	}
	return queryChaincodeRest(user, chaincodeInput)

}

func getAsset(user string) (asset string, err error) {
	myLogger.Debugf("Chaincode [queryAssetByOwner] args:[%s]-[%s]", "owner", user)
	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryAssetByOwner", user)}

	if connPeer == "grpc" {
		return queryChaincodeGrpc(user, chaincodeInput)
	}
	return queryChaincodeRest(user, chaincodeInput)

}

func getTxLogs() (txLogs string, err error) {
	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryTxLogs")}

	if connPeer == "grpc" {
		return queryChaincodeGrpc(admin, chaincodeInput)
	}
	return queryChaincodeRest(admin, chaincodeInput)
}

func initAccount(user string) (result string, err error) {
	myLogger.Debugf("Chaincode [initAccount] args:[%s]-[%s]", "initAccount", user)

	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("initAccount", user)}

	if connPeer == "grpc" {
		_, err = invokeChaincodeGrpc(user, adminInvoker, chaincodeInput)
		return
	}
	_, err = invokeChaincodeRest(user, chaincodeInput)
	return
}

func getMyReleaseLog(user string) (log string, err error) {
	myLogger.Debugf("Chaincode [getMyReleaseLog] args:[%s]-[%s]", "user", user)

	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryMyReleaseLog", user)}

	if connPeer == "grpc" {
		return queryChaincodeGrpc(user, chaincodeInput)
	}
	return queryChaincodeRest(user, chaincodeInput)
}

func getMyAssignLog(user string) (log string, err error) {
	myLogger.Debugf("Chaincode [getMyAssignLog] args:[%s]-[%s]", "user", user)

	chaincodeInput := &pb.ChaincodeInput{Args: util.ToChaincodeArgs("queryMyAssignLog", user)}

	if connPeer == "grpc" {
		return queryChaincodeGrpc(user, chaincodeInput)
	}
	return queryChaincodeRest(user, chaincodeInput)
}
