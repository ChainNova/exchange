package main

import (
	"encoding/json"
	"errors"

	pb "github.com/hyperledger/fabric/protos"
)

func deployChaincode(chaincode *Chaincode) (err error) {
	myLogger.Debug("------------- deploy chaincode -------------")

	reqBody, err := json.Marshal(chaincode)
	if err != nil {
		myLogger.Errorf("Failed deploying [%s]", err)
		return
	}

	respBody, err := doHTTPPost(adaptorURL+"/deploy", reqBody)
	if err != nil {
		myLogger.Errorf("Failed deploying [%s]", err)
		return
	}
	myLogger.Debugf("Resp [%s]", string(respBody))

	var result pb.Response
	err = json.Unmarshal(respBody, result)
	if err != nil {
		myLogger.Errorf("Failed deploying [%s]", err)
		return
	}

	if result.Status != pb.Response_SUCCESS {
		myLogger.Errorf("Failed deploying [%s]", result.Msg)
		return errors.New(string(result.Msg))
	}

	chaincodeName = string(result.Msg)
	myLogger.Debugf("ChaincodeName [%s]", chaincodeName)

	myLogger.Debug("------------- deploy Done! -------------")

	return
}

func invokeChaincode(chaincode *Chaincode) (ret string, err error) {
	myLogger.Debug("------------- invoke chainde -------------")

	reqBody, err := json.Marshal(chaincode)
	if err != nil {
		myLogger.Errorf("Failed invoke [%s]", err)
		return
	}

	respBody, err := doHTTPPost(adaptorURL+"/invoke", reqBody)
	if err != nil {
		myLogger.Errorf("Failed invoke [%s]", err)
		return
	}
	myLogger.Debugf("Resp [%s]", string(respBody))

	var result pb.Response
	err = json.Unmarshal(respBody, result)
	if err != nil {
		myLogger.Errorf("Failed invoke [%s]", err)
		return
	}

	if result.Status != pb.Response_SUCCESS {
		myLogger.Errorf("Failed invoke [%s]", result.Msg)
		err = errors.New(string(result.Msg))
		return
	}

	myLogger.Debug("------------- invoke chainde Done! -------------")

	ret = string(result.Msg)
	return
}

func queryChaincode(chaincode *Chaincode) (ret string, err error) {
	myLogger.Debug("------------- query chainde -------------")

	reqBody, err := json.Marshal(chaincode)
	if err != nil {
		myLogger.Errorf("Failed query [%s]", err)
		return
	}

	respBody, err := doHTTPPost(adaptorURL+"/query", reqBody)
	if err != nil {
		myLogger.Errorf("Failed query [%s]", err)
		return
	}
	myLogger.Debugf("Resp [%s]", string(respBody))

	var result pb.Response
	err = json.Unmarshal(respBody, result)
	if err != nil {
		myLogger.Errorf("Failed query [%s]", err)
		return
	}

	if result.Status != pb.Response_SUCCESS {
		myLogger.Errorf("Failed query [%s]", result.Msg)
		err = errors.New(string(result.Msg))
		return
	}

	myLogger.Debug("------------- query chainde Done! -------------")

	ret = string(result.Msg)
	return
}

func login(user *pb.Secret) (err error) {
	myLogger.Debug("------------- login -------------")

	reqBody, _ := json.Marshal(user)
	respBody, err := doHTTPPost(adaptorURL+"/login", reqBody)
	if err != nil {
		myLogger.Errorf("Failed login [%s]", err)
		return
	}
	myLogger.Debugf("Resp [%s]", string(respBody))

	var result pb.Response
	err = json.Unmarshal(respBody, result)
	if err != nil {
		myLogger.Errorf("Failed login [%s]", err)
		return
	}

	if result.Status != pb.Response_SUCCESS {
		myLogger.Errorf("Failed login [%s]", result.Msg)
		return
	}

	myLogger.Infof("Successful login [%s]", result.Msg)
	myLogger.Debug("------------- login Done! -------------")

	return
}
