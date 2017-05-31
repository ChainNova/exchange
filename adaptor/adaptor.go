package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gocraft/web"
	pb "github.com/hyperledger/fabric/protos"
)

// Adaptor defines the Openchain REST service object.
type Adaptor struct {
}

// NotFound NotFound
func (a *Adaptor) NotFound(rw web.ResponseWriter, req *web.Request) {
	rw.WriteHeader(http.StatusNotFound)
	json.NewEncoder(rw).Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte("Request not found")})
}

// SetResponseType is a middleware function that sets the appropriate response
// headers. Currently, it is setting the "Content-Type" to "application/json" as
// well as the necessary headers in order to enable CORS for Swagger usage.
func (a *Adaptor) SetResponseType(rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
	rw.Header().Set("Content-Type", "application/json")

	// Enable CORS
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "accept, content-type")

	next(rw, req)
}

// Login Login
func (a *Adaptor) Login(rw web.ResponseWriter, req *web.Request) {
	encoder := json.NewEncoder(rw)

	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte("Internal JSON-RPC error when reading request body")})
		myLogger.Errorf("Internal JSON-RPC error when reading request body: %s", err)
		return
	}
	// Incoming request body may not be empty, client must supply request payload
	if string(reqBody) == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte("Client must supply a payload for login requests")})
		myLogger.Error("Client must supply a payload for login requests.")
		return
	}
	myLogger.Debugf("Login req body: %s", string(reqBody))

	var user pb.Secret
	err = json.Unmarshal(reqBody, &user)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error unmarshalling login request payload: %s", err))})
		myLogger.Errorf("Error unmarshalling login request payload: %s", err)
		return
	}

	if user.EnrollId == "" || user.EnrollSecret == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte("Client must supply a uesr name and secret for login requests")})
		myLogger.Error("Client must supply a uesr name and secret for login requests.")
		return
	}

	_, err = setCryptoClient(user.EnrollId, user.EnrollSecret)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error set client: %s", err))})
		myLogger.Errorf("Error set client: %s", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(pb.Response{Status: pb.Response_SUCCESS, Msg: []byte(user.EnrollId)})
}

// Deploy Deploy
func (a *Adaptor) Deploy(rw web.ResponseWriter, req *web.Request) {
	encoder := json.NewEncoder(rw)

	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte("Internal JSON-RPC error when reading request body")})
		myLogger.Errorf("Internal JSON-RPC error when reading request body: %s", err)
		return
	}
	// Incoming request body may not be empty, client must supply request payload
	if string(reqBody) == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte("Client must supply a payload for chaincode requests")})
		myLogger.Error("Client must supply a payload for chaincode requests.")
		return
	}
	myLogger.Debugf("Deploy req body: %s", string(reqBody))

	var chaincode Chaincode
	err = json.Unmarshal(reqBody, &chaincode)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error unmarshalling chaincode request payload: %s", err))})
		myLogger.Errorf("Error unmarshalling chaincode request payload: %s", err)
		return
	}

	if chaincode.ID.Path == "" || chaincode.User.EnrollId == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte("Client must supply a chaincode path and uesr name  for chaincode requests")})
		myLogger.Error("Client must supply a chaincode path, uesr name and secret for chaincode requests.")
		return
	}

	chaincode.invoker, err = setCryptoClient(chaincode.User.EnrollId, chaincode.User.EnrollSecret)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error set client: %s", err))})
		myLogger.Errorf("Error set client: %s", err)
		return
	}

	resp, err := chaincode.deploy()
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error deploying chaincode: %s", err))})
		myLogger.Errorf("Error deploying chaincode: %s", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(resp)
}

// Invoke Invoke
func (a *Adaptor) Invoke(rw web.ResponseWriter, req *web.Request) {
	encoder := json.NewEncoder(rw)

	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte("Internal JSON-RPC error when reading request body")})
		myLogger.Errorf("Internal JSON-RPC error when reading request body: %s", err)
		return
	}

	// Incoming request body may not be empty, client must supply request payload
	if string(reqBody) == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte("Client must supply a payload for chaincode requests")})
		myLogger.Error("Client must supply a payload for chaincode requests.")
		return
	}
	myLogger.Debugf("Invoke req body: %s", string(reqBody))

	var chaincode Chaincode
	err = json.Unmarshal(reqBody, &chaincode)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error unmarshalling chaincode request payload: %s", err))})
		myLogger.Errorf("Error unmarshalling chaincode request payload: %s", err)
		return
	}

	if chaincode.ID.Name == "" || chaincode.User.EnrollId == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte("Client must supply a chaincode name and uesr name for chaincode requests")})
		myLogger.Error("Client must supply a chaincode name and uesr name for chaincode requests.")
		return
	}

	chaincode.invoker, err = setCryptoClient(chaincode.User.EnrollId, chaincode.User.EnrollSecret)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error set client: %s", err))})
		myLogger.Errorf("Error set client: %s", err)
		return
	}

	resp, err := chaincode.invoke()
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error invoking chaincode: %s", err))})
		myLogger.Errorf("Error invoking chaincode: %s", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(resp)
}

// Query Query
func (a *Adaptor) Query(rw web.ResponseWriter, req *web.Request) {
	encoder := json.NewEncoder(rw)

	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte("Internal JSON-RPC error when reading request body")})
		myLogger.Errorf("Internal JSON-RPC error when reading request body: %s", err)
		return
	}

	// Incoming request body may not be empty, client must supply request payload
	if string(reqBody) == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte("Client must supply a payload for chaincode requests")})
		myLogger.Error("Client must supply a payload for chaincode requests.")
		return
	}
	myLogger.Debugf("Invoke req body: %s", string(reqBody))

	var chaincode Chaincode
	err = json.Unmarshal(reqBody, &chaincode)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error unmarshalling chaincode request payload: %s", err))})
		myLogger.Errorf("Error unmarshalling chaincode request payload: %s", err)
		return
	}

	if chaincode.ID.Name == "" || chaincode.User.EnrollId == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte("Client must supply a chaincode name and uesr name for chaincode requests")})
		myLogger.Error("Client must supply a chaincode name and uesr name for chaincode requests.")
		return
	}

	chaincode.invoker, err = setCryptoClient(chaincode.User.EnrollId, chaincode.User.EnrollSecret)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error set client: %s", err))})
		myLogger.Errorf("Error set client: %s", err)
		return
	}

	resp, err := chaincode.query()
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(pb.Response{Status: pb.Response_FAILURE, Msg: []byte(fmt.Sprintf("Error query chaincode: %s", err))})
		myLogger.Errorf("Error query chaincode: %s", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(resp)
}
