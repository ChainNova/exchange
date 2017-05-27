package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/chaincode/platforms"
	"github.com/hyperledger/fabric/core/container"
	"github.com/hyperledger/fabric/core/crypto"
	"github.com/hyperledger/fabric/core/peer"
	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/protos"
	logging "github.com/op/go-logging"
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
	confidentialityOn    bool
	confidentialityLevel pb.ConfidentialityLevel

	serverClient pb.PeerClient
)

func initPeerClient() (err error) {
	viper.Set("ledger.blockchain.deploy-system-chaincode", "false")
	viper.Set("peer.validator.validity-period.verification", "false")

	peerClientConn, err := peer.NewPeerClientConnection()
	if err != nil {
		fmt.Printf("Error connection to server at host:port = %s\n", viper.GetString("peer.address"))
		return
	}
	serverClient = pb.NewPeerClient(peerClientConn)

	// Logging
	var formatter = logging.MustStringFormatter(
		`%{color}[%{module}] %{shortfunc} [%{shortfile}] -> %{level:.4s} %{id:03x}%{color:reset} %{message}`,
	)
	logging.SetFormatter(formatter)

	return
}

func setCryptoClient(enrollID, enrollPWD string) (crypto.Client, error) {
	if len(enrollPWD) > 0 {
		if err := crypto.RegisterClient(enrollID, nil, enrollID, enrollPWD); err != nil {
			return nil, err
		}
	}

	client, err := crypto.InitClient(enrollID, nil)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func confidentiality(enabled bool) {
	confidentialityOn = enabled

	if confidentialityOn {
		confidentialityLevel = pb.ConfidentialityLevel_CONFIDENTIAL
	} else {
		confidentialityLevel = pb.ConfidentialityLevel_PUBLIC
	}
}

func (c *Chaincode) deploy() (resp *pb.Response, err error) {
	myLogger.Debug("*********** deploy chainde *************")

	// Prepare the spec. The metadata includes the identity of the administrator
	spec := &pb.ChaincodeSpec{
		Type:        c.Type,
		ChaincodeID: c.ID,
		CtorMsg:     c.Input,
		// Metadata:             adminCert.GetCertificate(),
		// SecureContext:
		ConfidentialityLevel: confidentialityLevel,
	}

	// First build the deployment spec
	cds, err := getChaincodeBytes(spec)
	if err != nil {
		return nil, fmt.Errorf("Error getting deployment spec: %s ", err)
	}

	// Now create the Transactions message and send to Peer.
	transaction, err := c.invoker.NewChaincodeDeployTransaction(cds, cds.ChaincodeSpec.ChaincodeID.Name)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}

	resp, err = processTransaction(transaction)
	if err != nil {
		return nil, fmt.Errorf("Error deploy chaincode: %s ", err)
	}
	myLogger.Debugf("resp [%s]", resp.String())

	if resp.Status != pb.Response_SUCCESS {
		return nil, fmt.Errorf("Error deploy chaincode: %s ", string(resp.Msg))
	}

	return
}

func (c *Chaincode) invoke() (resp *pb.Response, err error) {
	myLogger.Debug("------------- invoke chainde -------------")

	// Get a transaction handler to be used to submit the execute transaction
	// and bind the chaincode access control logic using the binding
	submittingCertHandler, err := c.invoker.GetTCertificateHandlerNext()
	if err != nil {
		return nil, err
	}
	txHandler, err := submittingCertHandler.GetTransactionHandler()
	if err != nil {
		return nil, err
	}
	// binding, err := txHandler.GetBinding()
	// if err != nil {
	// 	return nil, err
	// }

	// chaincodeInputRaw, err := proto.Marshal(chaincodeInput)
	// if err != nil {
	// 	return nil, err
	// }

	// Access control. Administrator signs chaincodeInputRaw || binding to confirm his identity
	// sigma, err := invokerCert.Sign(append(chaincodeInputRaw, binding...))
	// if err != nil {
	// 	return nil, err
	// }

	// Prepare spec and submit
	spec := &pb.ChaincodeSpec{
		Type:        c.Type,
		ChaincodeID: c.ID,
		CtorMsg:     c.Input,
		// Metadata:             sigma, // Proof of identity
		// SecureContext:        secureContext,
		ConfidentialityLevel: confidentialityLevel,
	}

	chaincodeInvocationSpec := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	// Now create the Transactions message and send to Peer.
	transaction, err := txHandler.NewChaincodeExecute(chaincodeInvocationSpec, util.GenerateUUID())
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}

	resp, err = processTransaction(transaction)
	if err != nil {
		return nil, fmt.Errorf("Error invoking chaincode: %s ", err)
	}
	myLogger.Debugf("Resp [%s]", resp.String())

	if resp.Status != pb.Response_SUCCESS {
		return nil, fmt.Errorf("Error invoke chaincode: %s ", string(resp.Msg))
	}

	return
}

func (c *Chaincode) query() (resp *pb.Response, err error) {
	myLogger.Debug("############# query chainde ###########")

	// Prepare spec and submit
	spec := &pb.ChaincodeSpec{
		Type:        c.Type,
		ChaincodeID: c.ID,
		CtorMsg:     c.Input,
		// SecureContext:        secureContext,
		ConfidentialityLevel: confidentialityLevel,
	}

	chaincodeInvocationSpec := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	// Now create the Transactions message and send to Peer.
	transaction, err := c.invoker.NewChaincodeQuery(chaincodeInvocationSpec, util.GenerateUUID())
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}

	resp, err = processTransaction(transaction)
	myLogger.Debugf("Resp [%s]", resp.String())

	if confidentialityOn {
		resp.Msg, err = c.invoker.DecryptQueryResult(transaction, resp.Msg)
		if err != nil {
			return nil, fmt.Errorf("Decrypt Query Result error:%s", err)
		}
	}
	myLogger.Debugf("Resp [%s]", resp.String())

	if resp.Status != pb.Response_SUCCESS || string(resp.Msg) == "null" {
		return nil, fmt.Errorf("Error query chaincode: %s ", string(resp.Msg))
	}

	return
}

func getChaincodeBytes(spec *pb.ChaincodeSpec) (*pb.ChaincodeDeploymentSpec, error) {
	var codePackageBytes []byte

	if viper.GetString("chaincode.mode") != chaincode.DevModeUserRunsChaincode {
		myLogger.Debugf("Received build request for chaincode spec: %v", spec)
		var err error
		if err = checkSpec(spec); err != nil {
			return nil, err
		}

		codePackageBytes, err = container.GetChaincodePackageBytes(spec)
		if err != nil {
			err = fmt.Errorf("Error getting chaincode package bytes: %s", err)
			myLogger.Errorf("%s", err)
			return nil, err
		}
	}
	chaincodeDeploymentSpec := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec, CodePackage: codePackageBytes}

	return chaincodeDeploymentSpec, nil
}

func checkSpec(spec *pb.ChaincodeSpec) error {
	// Don't allow nil value
	if spec == nil {
		return errors.New("Expected chaincode specification, nil received")
	}

	platform, err := platforms.Find(spec.Type)
	if err != nil {
		return fmt.Errorf("Failed to determine platform type: %s", err)
	}

	return platform.ValidateSpec(spec)
}

func processTransaction(tx *pb.Transaction) (*pb.Response, error) {
	return serverClient.ProcessTransaction(context.Background(), tx)
}
