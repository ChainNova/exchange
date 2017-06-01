package main

import (
	"encoding/json"
	"fmt"

	"time"

	"github.com/hyperledger/fabric/events/consumer"
	pb "github.com/hyperledger/fabric/protos"
	"github.com/spf13/viper"
)

type adapter struct {
	blockEvent     chan *pb.Event_Block
	chaincodeEvent chan *pb.Event_ChaincodeEvent
	rejectionEvent chan *pb.Event_Rejection
	chaincodeID    string
}
type FailInfo struct {
	Id   string `json:"id"`
	Info string `json:"info"`
}

type BatchResult struct {
	EventName string     `json:"eventName"`
	SrcMethod string     `json:"srcMethod"`
	Success   []string   `json:""success`
	Fail      []FailInfo `json:"fail"`
}

const Chaincode_Success = "SUCCESS"

var (
	chaincodeID string
	obcEHClient *consumer.EventsClient
)

func (a *adapter) GetInterestedEvents() ([]*pb.Interest, error) {
	if a.chaincodeID != "" {
		return []*pb.Interest{
			&pb.Interest{EventType: pb.EventType_BLOCK},
			&pb.Interest{EventType: pb.EventType_REJECTION},
			&pb.Interest{EventType: pb.EventType_CHAINCODE,
				RegInfo: &pb.Interest_ChaincodeRegInfo{
					ChaincodeRegInfo: &pb.ChaincodeReg{
						ChaincodeID: a.chaincodeID,
						EventName:   "",
					},
				},
			},
		}, nil
	}
	return []*pb.Interest{
		&pb.Interest{EventType: pb.EventType_BLOCK},
		&pb.Interest{EventType: pb.EventType_REJECTION},
	}, nil
}

func (a *adapter) Recv(msg *pb.Event) (bool, error) {
	if e, o := msg.Event.(*pb.Event_Block); o {
		a.blockEvent <- e
		return true, nil
	} else if e, o := msg.Event.(*pb.Event_ChaincodeEvent); o {
		a.chaincodeEvent <- e
		return true, nil
	} else if e, o := msg.Event.(*pb.Event_Rejection); o {
		a.rejectionEvent <- e
		return true, nil
	}

	return false, fmt.Errorf("Receive unkown type event: %v", msg)
}

func (a *adapter) Disconnected(err error) {
	myLogger.Debug("Disconnected...reconnecting\n")
	obcEHClient.Stop()
	go eventListener()
	myLogger.Debug("Reconnected...\n")
}

func eventListener() {
	eventAddress := viper.GetString("event.address")

	chaincodeID, _ = getChaincodeID()
	if chaincodeID == "" {
		myLogger.Error("Can't find chaincode!!!")
		return
	}

	a := &adapter{
		blockEvent:     make(chan *pb.Event_Block),
		chaincodeEvent: make(chan *pb.Event_ChaincodeEvent),
		rejectionEvent: make(chan *pb.Event_Rejection),
		chaincodeID:    chaincodeID}

	t := viper.GetDuration("event.client.regTimeout")
	obcEHClient, _ = consumer.NewEventsClient(eventAddress, t, a)
	for {
		if err := obcEHClient.Start(); err != nil {
			myLogger.Errorf("could not start chat: %s, reconnecting...\n", err)
			obcEHClient.Stop()
		} else {
			myLogger.Errorf("connected eventAddress %v ok\n", eventAddress)
			break
		}
	}

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

// 定时检测chaincodeID是否变化过，如果变化，则重新监听新的chaincode事件
func checkChaincodeID() {
	ticker := time.NewTicker(viper.GetDuration("event.chaincode.check"))

	for _ = range ticker.C {
		newChaincodeID, _ := getChaincodeID()
		if newChaincodeID != chaincodeID {
			myLogger.Debug("chaincode changes...reconnecting\n")
			obcEHClient.Stop()
			go eventListener()
			myLogger.Debug("Reconnected...\n")
		}
	}
}
