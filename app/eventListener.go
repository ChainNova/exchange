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

var (
	chaincodeResult      = make(map[string]string)      //存储chaincode执行结果，key为txid，value为结果，最终成功以blockEvent为准
	chaincodeBatchResult = make(map[string]BatchResult) //存储chaincode批量操作结果，key为txid，value为结果。该内容与chaincodeResult结合确定最终执行结果
)

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
	go eventListener(a.chaincodeID)
	myLogger.Debug("Reconnected...\n")
}

func eventListener(chaincodeID string) {
	eventAddress := viper.GetString("peer.validator.events.address")

	a := &adapter{
		blockEvent:     make(chan *pb.Event_Block),
		chaincodeEvent: make(chan *pb.Event_ChaincodeEvent),
		rejectionEvent: make(chan *pb.Event_Rejection),
		chaincodeID:    chaincodeID}

	obcEHClient, _ := consumer.NewEventsClient(eventAddress, 50*time.Second, a)
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
				chaincodeResult[r.Txid] = Chaincode_Success
				dealResult(r.Txid)
			}
		case r := <-a.rejectionEvent:
			myLogger.Debug("Received rejected transaction\n")
			myLogger.Debug("--------------\n")
			myLogger.Debugf("Transaction error:\n%s\n", r.Rejection.ErrorMsg)

			if r.Rejection.Tx != nil {
				chaincodeResult[r.Rejection.Tx.Txid] = r.Rejection.ErrorMsg
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
			chaincodeBatchResult[ce.ChaincodeEvent.TxID] = batch
			dealResult(ce.ChaincodeEvent.TxID)
		}
	}
}

// 非批量操作的结果用chaincodeResult[txid]即可处理
// 批量操作的结果由两种
// 1.成功：通过chaincodeResult[txid]=success 和 chaincodeBatchResult[txid].Success[] 来确定
// 2.失败：a. chaincode里直接return err的失败，这种失败保存在chaincodeResult[txid]=ErrMsg中，表示整批操作全部失败.这种失败不处理失败成员
// 		  b. chaincodeBatchResult[txid].Fail[]里的失败，表示批量处理部分失败（校验失败），这种失败是处理失败成员
func dealResult(txid string) {
	r1, ok1 := chaincodeBatchResult[txid]
	r2, ok2 := chaincodeResult[txid]

	if ok1 && ok2 {
		if r2 == Chaincode_Success {
			switch r1.EventName {
			case "chaincode_lock":
				if r1.SrcMethod == "lock" {
					lockSuccess(r1.Success)
					lockFail(r1.Fail)
				} else if r1.SrcMethod == "expire" {
					expiredSuccess(r1.Success)
					expiredFail(r1.Fail)
				} else if r1.SrcMethod == "cancel" {
					cancelSuccess(r1.Success)
					cancelFailed(r1.Fail)
				}
			case "chaincode_exchange":
				execTxSuccess(r1.Success)
				execTxFail(r1.Fail)
			}
		}
	}
}
