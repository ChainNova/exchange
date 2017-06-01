package main

import (
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
	eventListener(a)
	myLogger.Debug("Reconnected...\n")
}

func eventListener(a *adapter) {
	eventAddress := viper.GetString("event.address")
	a.chaincodeID = chaincodeID

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
}

// 定时检测chaincodeID是否变化过，如果变化，则重新监听新的chaincode事件
func checkChaincodeID() {
	ticker := time.NewTicker(viper.GetDuration("event.chaincode.check"))

	for _ = range ticker.C {
		newChaincodeID, _ := getChaincodeID()
		if newChaincodeID != chaincodeID {
			myLogger.Debugf("chaincode changed: %s-->%s...reconnecting", chaincodeID, newChaincodeID)
			obcEHClient.Stop()
			chaincodeID = newChaincodeID
			eventListener(a)
			myLogger.Debug("Reconnected...\n")
		}
	}
}
