package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gocraft/web"
	"github.com/hyperledger/fabric/core/util"
	"github.com/spf13/viper"
)

type restResp struct {
	Status string      `json:"status"`
	Result interface{} `json:"result"`
}

type respErr struct {
	Code string `json:"code"`
	Msg  string `json:"mag"`
}

type txResult struct {
	Txid string `json:"txid"`
}

type checkResult struct {
	Flag string `json:"flag"`
}

type User struct {
	EnrollID     string `json:"enrollId"`
	EnrollSecret string `json:"enrollSecret"`
}

const (
	SUCCESS     = "success"
	FAILED      = "failed"
	SYSERR      = "SYS_ERR"
	NOTLOGIN    = "NOT_LOGIN"
	PARAMERR    = "REQ_PARAM_ERR"
	REQNOTFOUND = "REQ_NOT_FOUND"
)

// Order Order
type Order struct {
	UUID         string  `json:"uuid"`        //UUID
	Account      string  `json:"account"`     //账户
	SrcCurrency  string  `json:"srcCurrency"` //源币种代码
	SrcCount     float64 `json:"srcCount"`    //源币种交易数量
	DesCurrency  string  `json:"desCurrency"` //目标币种代码
	DesCount     float64 `json:"desCount"`    //目标币种交易数量
	IsBuyAll     bool    `json:"isBuyAll"`    //是否买入所有，即为true是以目标币全部兑完为主,否则算部分成交,买完为止；为false则是以源币全部兑完为主,否则算部分成交，卖完为止
	ExpiredTime  int64   `json:"expiredTime"` //超时时间
	ExpiredDate  string  `json:"expiredDate"`
	PendingTime  int64   `json:"PendingTime"` //挂单时间
	PendingDate  string  `json:"pendingDate"`
	PendedTime   int64   `json:"PendedTime"` //挂单完成时间
	PendedDate   string  `json:"pendedDate"`
	MatchedTime  int64   `json:"matchedTime"` //撮合完成时间
	MatchedDate  string  `json:"matchedDate"`
	MatchedUUID  string  `json:"matchedUUID"`  //与之撮合的挂单
	FinishedTime int64   `json:"finishedTime"` //交易完成时间
	FinishedDate string  `json:"finishedDate"`
	RawUUID      string  `json:"rawUUID"`     //母单UUID
	RawSrcCount  float64 `json:"rawSrcCount"` //母单源币数数量（因为拆分挂单时，母单数量会被修改，所以记录下来方便校对）
	RawDesCount  float64 `json:"rawDesCount"` //母单目标币数数量（因为拆分挂单时，母单数量会被修改，所以记录下来方便校对）
	Metadata     string  `json:"metadata"`    //存放其他数据，如挂单锁定失败信息
	FinalCost    float64 `json:"finalCost"`   //源币的最终消耗数量，主要用于买完（IsBuyAll=true）的最后一笔交易计算结余，此时SrcCount有可能大于FinalCost
	Status       int     `json:"status"`      //状态 0：待交易，1：完成，2：过期，3：撤单
}

// Order Order
type OrderInt struct {
	UUID         string `json:"uuid"`         //UUID
	Account      string `json:"account"`      //账户
	SrcCurrency  string `json:"srcCurrency"`  //源币种代码
	SrcCount     int64  `json:"srcCount"`     //源币种交易数量
	DesCurrency  string `json:"desCurrency"`  //目标币种代码
	DesCount     int64  `json:"desCount"`     //目标币种交易数量
	IsBuyAll     bool   `json:"isBuyAll"`     //是否买入所有，即为true是以目标币全部兑完为主,否则算部分成交,买完为止；为false则是以源币全部兑完为主,否则算部分成交，卖完为止
	ExpiredTime  int64  `json:"expiredTime"`  //超时时间
	PendingTime  int64  `json:"PendingTime"`  //挂单时间
	PendedTime   int64  `json:"PendedTime"`   //挂单完成时间
	MatchedTime  int64  `json:"matchedTime"`  //撮合完成时间
	FinishedTime int64  `json:"finishedTime"` //交易完成时间
	RawUUID      string `json:"rawUUID"`      //母单UUID
	Metadata     string `json:"metadata"`     //存放其他数据，如挂单锁定失败信息
	FinalCost    int64  `json:"finalCost"`    //源币的最终消耗数量，主要用于买完（IsBuyAll=true）的最后一笔交易计算结余，此时SrcCount有可能大于FinalCost
	Status       int    `json:"status"`       //状态
}

type Currency struct {
	ID         string  `json:"id"`
	Count      float64 `json:"count"`
	LeftCount  float64 `json:"leftCount"`
	Creator    string  `json:"creator"`
	User       string  `json:"user"`
	CreateTime int64   `json:"createTime"`
}

type Null struct {
}

var Multiple = math.Pow10(6)

// NotFound NotFound
func (a *AppREST) NotFound(rw web.ResponseWriter, req *web.Request) {
	rw.WriteHeader(http.StatusNotFound)
	json.NewEncoder(rw).Encode(restResp{Status: FAILED, Result: respErr{Code: REQNOTFOUND, Msg: "Request not found"}})
}

// SetResponseType is a middleware function that sets the appropriate response
// headers. Currently, it is setting the "Content-Type" to "application/json" as
// well as the necessary headers in order to enable CORS for Swagger usage.
func (s *AppREST) SetResponseType(rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
	rw.Header().Set("Content-Type", "application/json")

	// Enable CORS
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "accept, content-type")

	next(rw, req)
}

// Create 创建币
func (a *AppREST) Create(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing create currency request...")

	encoder := json.NewEncoder(rw)

	enrollID, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("Create failed: [%s].", err)
		return
	}

	// Read in the incoming request payload
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Internal JSON error when reading request body"}})
		myLogger.Error("Internal JSON error when reading request body.")
		return
	}

	// Incoming request body may not be empty, client must supply request payload
	if string(reqBody) == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Client must supply a payload for order requests"}})
		myLogger.Error("Client must supply a payload for order requests.")
		return
	}
	myLogger.Debugf("Req body: %s", string(reqBody))

	// Payload must conform to the following structure
	var currency Currency

	// Decode the request payload as an Request structure.	There will be an
	// error here if the incoming JSON is invalid
	err = json.Unmarshal(reqBody, &currency)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "request parameter is wrong"}})
		myLogger.Errorf("Error unmarshalling order request payload: %s", err)
		return
	}

	// 校验请求数据
	if len(currency.ID) <= 0 {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Currency cann't be empty"}})
		myLogger.Error("Currency cann't be empty.")
		return
	}
	if currency.Count < 0 {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Count must be greater than 0"}})
		myLogger.Error("Count must be greater than 0.")
		return
	}

	currency.User = enrollID
	// chaincode
	txid, err := createCurrency(currency.ID, int64(round(currency.Count, 6)*Multiple), currency.User)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "create Currency failed"}})
		myLogger.Errorf("create Currency failed:%s", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: txResult{Txid: txid}})

	myLogger.Debug("------------- Create Done")
}

// CheckCreate  检测创建币结果，由前端轮询
// response说明：StatusBadRequest  失败  不需继续轮询，Error表示失败原因
//				StatusOK OK="1" 成功  不需继续轮询
//				StatusOK OK="0" 未果  需要继续轮询
func (a *AppREST) CheckCreate(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing check create request...")

	encoder := json.NewEncoder(rw)

	_, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("CheckCreate failed: [%s].", err)
		return
	}

	txid := req.PathParams["txid"]
	if txid == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Client must supply a id for checkcreate requests"}})
		myLogger.Errorf("Client must supply a id for checkcreate requests.")
		return
	}
	myLogger.Debugf("check create request parameter:txid = %s", txid)

	v, ok := chaincodeResult[txid]
	if !ok {
		rw.WriteHeader(http.StatusOK)
		encoder.Encode(restResp{Status: SUCCESS, Result: checkResult{Flag: "0"}})
	} else if v == Chaincode_Success {
		rw.WriteHeader(http.StatusOK)
		encoder.Encode(restResp{Status: SUCCESS, Result: checkResult{Flag: "1"}})
	} else {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: v}})
	}

	myLogger.Debug("------------- CheckCreate Done")
}

// Currency 获取币信息
func (a *AppREST) Currency(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing get currency request...")

	encoder := json.NewEncoder(rw)

	_, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("Currency failed: [%s].", err)
		return
	}

	id := req.PathParams["id"]
	if id == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Currency id can't be empty"}})
		myLogger.Error("Get currency failed:Currency id can't be empty")
		return
	}
	myLogger.Debugf("Get currency parameter id = %s", id)

	result, _ := getCurrency(id)
	// if err != nil {
	// 	rw.WriteHeader(http.StatusBadRequest)
	// 	encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "Get currency failed"}})
	// 	myLogger.Errorf("Get currency failed:%s", err)
	// 	return
	// }
	if len(result) == 0 {
		rw.WriteHeader(http.StatusOK)
		encoder.Encode(restResp{Status: SUCCESS, Result: struct {
			Null `json:"currency"`
		}{Null{}}})
		return
	}

	var currency Currency
	err = json.Unmarshal([]byte(result), &currency)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "Get currency failed"}})
		myLogger.Errorf("Get currency failed:%s", err)
		return
	}

	currency.Count = currency.Count / Multiple
	currency.LeftCount = currency.LeftCount / Multiple

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: struct {
		Currency `json:"currency"`
	}{currency}})

	myLogger.Debug("------------- Currency Done")
}

// Currencys 获取币信息
func (a *AppREST) Currencys(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing get all currency request...")

	encoder := json.NewEncoder(rw)

	_, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("Currencys failed: [%s].", err)
		return
	}

	result, _ := getCurrencys()
	// if err != nil {
	// 	rw.WriteHeader(http.StatusBadRequest)
	// 	encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "Get currency failed"}})
	// 	myLogger.Errorf("Get currency failed: %s", err)
	// 	return
	// }

	currencys := []*Currency{}
	if len(result) > 0 {
		err = json.Unmarshal([]byte(result), &currencys)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "Get currency failed"}})
			myLogger.Errorf("Get currency failed : %s", err)
			return
		}

		for k, v := range currencys {
			currencys[k].Count = v.Count / Multiple
			currencys[k].LeftCount = v.LeftCount / Multiple
		}
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: struct {
		Currencys []*Currency `json:"currencys"`
	}{Currencys: currencys}})

	myLogger.Debug("------------- Currencys Done")
}

// MyCurrency MyCurrency
func (a *AppREST) MyCurrency(rw web.ResponseWriter, req *web.Request) {
	myLogger.Debug("------------- MyCurrency...")

	encoder := json.NewEncoder(rw)
	enrollID, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("MyCurrency failed: [%s]", err)
		return
	}

	// 获取个人币
	result, _ := getCurrencysByUser(enrollID)
	// if err != nil {
	// 	rw.WriteHeader(http.StatusBadRequest)
	// 	encoder.Encode(restResp{Status: FAILED, Msg: respErr{Code: SYSERR, Msg: "Get currency failed"}})
	// 	myLogger.Errorf("Get currency failed")
	// 	return
	// }

	myCurrency := []*Currency{}
	if len(result) > 0 {
		_ = json.Unmarshal([]byte(result), &myCurrency)
		// if err != nil {
		// 	rw.WriteHeader(http.StatusBadRequest)
		// 	encoder.Encode(restResp{Status: FAILED, Msg: respErr{Code: SYSERR, Msg: "Get currency failed"}})
		// 	myLogger.Errorf("Get currency failed")
		// 	return
		// }

		for k, v := range myCurrency {
			myCurrency[k].Count = v.Count / Multiple
			myCurrency[k].LeftCount = v.LeftCount / Multiple
		}
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{
		Status: SUCCESS,
		Result: struct {
			Currencys []*Currency `json:"currencys"`
		}{
			Currencys: myCurrency,
		},
	})

	myLogger.Debug("------------- MyCurrency Done")
}

// MyAsset MyAsset
func (a *AppREST) MyAsset(rw web.ResponseWriter, req *web.Request) {
	myLogger.Debug("------------- mMyAssety...")

	encoder := json.NewEncoder(rw)
	enrollID, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("MyAsset failed: [%s].", err)
		return
	}

	// 获取个人资产
	result, _ := getAsset(enrollID)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "Get owner asset failed"}})
		myLogger.Errorf("Get owner asset failed")
		return
	}

	myAsset := []Asset{}
	if len(result) > 0 {
		_ = json.Unmarshal([]byte(result), &myAsset)
		// if err != nil {
		// 	rw.WriteHeader(http.StatusBadRequest)
		// 	encoder.Encode(restResp{Status: FAILED, Msg: respErr{Code: SYSERR, Msg: "Get owner asset failed"}})
		// 	myLogger.Errorf("Get owner asset failed")
		// 	return
		// }

		for k, v := range myAsset {
			myAsset[k].Count = v.Count / Multiple
			myAsset[k].LockCount = v.LockCount / Multiple
		}
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{
		Status: SUCCESS,
		Result: struct {
			Assets []Asset `json:"assets"`
		}{
			myAsset,
		},
	})

	myLogger.Debugf("MyAsset successful for user '%s'.", enrollID)

	myLogger.Debug("------------- MyAsset Done")
}

type Asset struct {
	Owner     string  `json:"owner"`
	Currency  string  `json:"currency"`
	Count     float64 `json:"count"`
	LockCount float64 `json:"lockCount"`
}

// Tx 将挂单信息
func (a *AppREST) Tx(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing get tx request...")

	encoder := json.NewEncoder(rw)

	_, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("Tx failed: [%s].", err)
		return
	}

	tx, err := getOrder(req.PathParams["uuid"])
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "Get tx failed"}})
		myLogger.Errorf("Get tx faile:%s", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: struct {
		Order *Order `json:"order"`
	}{Order: tx}})

	myLogger.Debug("------------- Tx Done")
}

// MyTxs 个人挂单记录
func (a *AppREST) MyTxs(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing get user txs request...")

	encoder := json.NewEncoder(rw)

	enrollID, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("MyTxs failed: [%s].", err)
		return
	}

	txs, err := getOrderByUser(enrollID)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "Get txs failed"}})
		myLogger.Errorf("Get txs faile:%s", err)
		return
	}

	orders := []*Order{}
	status, err := strconv.Atoi(req.PathParams["status"])
	if err == nil {
		// 状态 0：待交易，1：完成，2：过期，3：撤单
		for _, v := range txs {
			if v.Status == status {
				orders = append(orders, v)
			}
		}
	} else if txs != nil && len(txs) > 0 {
		orders = txs
	}

	count, err := strconv.Atoi(req.PathParams["count"])
	if err == nil && count > 0 && count < len(orders) {
		orders = orders[:count]
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: struct {
		Orders []*Order `json:"orders"`
	}{Orders: orders}})

	myLogger.Debug("------------- MyTxs Done")
}

type History struct {
	Time string      `json:"time"`
	Type string      `json:"type"` // 建币  增币  分发币 接收币  订单
	Data interface{} `json:"data"`
}

// History 历史信息（包括创建币、增加币、分发币、接收币、挂单、撤单、过期）
func (a *AppREST) History(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing get user history request...")

	encoder := json.NewEncoder(rw)

	enrollID, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("History failed: [%s].", err)
		return
	}

	history := []*History{}

	// 挂单
	txs, _ := getOrderByUser(enrollID)
	for _, v := range txs {
		history = append(history,
			&History{
				Time: v.PendedDate,
				Type: "Exchange",
				Data: v,
			})
	}

	// 获取个人币
	result, _ := getCurrencysByUser(enrollID)
	myCurrency := []Currency{}
	if len(result) > 0 {
		json.Unmarshal([]byte(result), &myCurrency)

		for _, v := range myCurrency {
			v.Count = v.Count / Multiple
			v.LeftCount = v.LeftCount / Multiple

			history = append(history,
				&History{
					Time: time.Unix(v.CreateTime, 0).Format("2006-01-02 15:04:05"),
					Type: "CreateCurrency",
					Data: v,
				})
		}
	}

	// 增加币
	releaseLog, _ := getMyReleaseLog(enrollID)
	logRelease := []struct {
		Currency    string  `json:"currency"`
		Count       float64 `json:"cont"`
		ReleaseTime int64   `json:"releaseTime"`
	}{}
	if len(releaseLog) > 0 {
		json.Unmarshal([]byte(releaseLog), &logRelease)

		for _, v := range logRelease {
			v.Count = v.Count / Multiple

			history = append(history,
				&History{
					Time: time.Unix(v.ReleaseTime, 0).Format("2006-01-02 15:04:05"),
					Type: "ReleaseCurrency",
					Data: v,
				})
		}
	}

	// 分发币 // 接收币
	assignLog, _ := getMyAssignLog(enrollID)

	type Assign struct {
		Currency   string `json:"currency`
		Owner      string `json:"owner"`
		Count      int64  `json:"count"`
		AssignTime int64  `json:"assignTime"`
	}
	logAssign := struct {
		ToMe []*Assign `json:"toMe"`
		MeTo []*Assign `json:"meTo"`
	}{}
	if len(assignLog) > 0 {
		json.Unmarshal([]byte(assignLog), &logAssign)

		for _, v := range logAssign.MeTo {
			v.Count = int64(float64(v.Count) / Multiple)
			history = append(history,
				&History{
					Time: time.Unix(v.AssignTime, 0).Format("2006-01-02 15:04:05"),
					Type: "AssignCurrency_MeTo",
					Data: v,
				})
		}
		for _, v := range logAssign.ToMe {
			v.Count = int64(float64(v.Count) / Multiple)
			history = append(history,
				&History{
					Time: time.Unix(v.AssignTime, 0).Format("2006-01-02 15:04:05"),
					Type: "AssignCurrency_ToMe",
					Data: v,
				})
		}
	}

	sort.Sort(Historys(history))

	count, err := strconv.Atoi(req.PathParams["count"])
	if err == nil && count > 0 && count < len(history) {
		history = history[:count]
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: struct {
		Historys []*History `json:"historys"`
	}{Historys: history}})

	myLogger.Debug("------------- History Done")
}

// CurrencysTxs 两币之间挂单记录
func (a *AppREST) CurrencysTxs(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing get currency txs request...")

	encoder := json.NewEncoder(rw)

	enrollID, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("Get txs: [%s].", err)
		return
	}

	srcCurrency := req.PathParams["srccurrency"]
	desCurrency := req.PathParams["descurrency"]
	// if len(srcCurrency) == 0 || len(desCurrency) == 0 {
	// 	rw.WriteHeader(http.StatusBadRequest)
	// 	encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "request parameter srccurrency or descurrency can't be null"}})
	// 	myLogger.Errorf("request parameter srccurrency or descurrency can't be null")
	// 	return
	// }

	srcDesTxs, desSrcTxs := []*Order{}, []*Order{}
	txs, err := getOrderByUser(enrollID)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "Get txs failed"}})
		myLogger.Errorf("Get txs faile:%s", err)
		return
	}

	for _, v := range txs {
		if (len(srcCurrency) == 0 || v.SrcCurrency == srcCurrency) && (len(desCurrency) == 0 || v.DesCurrency == desCurrency) {
			srcDesTxs = append(srcDesTxs, v)
		}
		if (len(desCurrency) == 0 || v.SrcCurrency == desCurrency) && (len(srcCurrency) == 0 || v.DesCurrency == srcCurrency) {
			desSrcTxs = append(desSrcTxs, v)
		}
	}

	count, err := strconv.Atoi(req.PathParams["count"])
	// if err != nil {
	// 	rw.WriteHeader(http.StatusBadRequest)
	// 	encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "request parameter is wrong"}})
	// 	myLogger.Errorf("request parameter is wrong: %s", err)
	// 	return
	// }
	// if count < 0 {
	// 	rw.WriteHeader(http.StatusBadRequest)
	// 	encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "request parameter count must >= 0"}})
	// 	myLogger.Errorf("request parameter count must >= 0")
	// 	return
	// }
	if err == nil && count > 0 && count < len(srcDesTxs) {
		srcDesTxs = srcDesTxs[:count]
	}
	if err == nil && count > 0 && count < len(desSrcTxs) {
		desSrcTxs = desSrcTxs[:count]
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: struct {
		SrcDes []*Order `json:"srcDes"`
		DesSrc []*Order `json:"desSrc"`
	}{SrcDes: srcDesTxs, DesSrc: desSrcTxs}})

	myLogger.Debug("------------- CurrencysTxs Done")
}

// Market 市场挂单（未被撮合）
func (a *AppREST) Market(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing get market request...")

	encoder := json.NewEncoder(rw)

	_, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("Market failed: [%s].", err)
		return
	}

	var srcUuids, desUuids []string

	srcCurrency := req.PathParams["srccurrency"]
	desCurrency := req.PathParams["descurrency"]
	if len(srcCurrency) == 0 || len(desCurrency) == 0 {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "request parameter srccurrency or descurrency can't be null"}})
		myLogger.Errorf("request parameter srccurrency or descurrency can't be null")
		return
	}
	count, err := strconv.ParseInt(req.PathParams["count"], 10, 64)
	if err != nil || count == 0 {
		count = -1
	}
	srcUuids, _ = getRangeZSet(getBSKey(srcCurrency, desCurrency), count)
	desUuids, _ = getRangeZSet(getBSKey(desCurrency, srcCurrency), count)

	type Market struct {
		UUID  string  `json:"uuis"`
		Count float64 `json:"count"` //交易数量
		Price float64 `json:"price"` //价格
	}

	srcDesTxs, desSrcTxs := []*Market{}, []*Market{}

	for _, v := range srcUuids {
		o, err := getOrder(v)
		if err == nil {
			srcDesTxs = append(srcDesTxs,
				&Market{
					UUID:  o.UUID,
					Count: o.SrcCount,
					Price: round(o.DesCount/o.SrcCount, 6)})
		}
	}
	for _, v := range desUuids {
		o, err := getOrder(v)
		if err == nil {
			desSrcTxs = append(desSrcTxs,
				&Market{
					UUID:  o.UUID,
					Count: o.DesCount,
					Price: round(o.SrcCount/o.DesCount, 6)})
		}
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: struct {
		Ask []*Market `json:"ask"`
		Bid []*Market `json:"bid"`
	}{Ask: srcDesTxs, Bid: desSrcTxs}})

	myLogger.Debug("------------- Market Done")
}

// Release 发布币
func (a *AppREST) Release(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing currency release request...")

	encoder := json.NewEncoder(rw)

	enrollID, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("Release failed: [%s].", err)
		return
	}

	// Read in the incoming request payload
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Internal JSON error when reading request body"}})
		myLogger.Error("Internal JSON error when reading request body.")
		return
	}
	myLogger.Debugf("Req body: %s", string(reqBody))
	// Incoming request body may not be empty, client must supply request payload
	if string(reqBody) == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Client must supply a payload for order requests"}})
		myLogger.Error("Client must supply a payload for order requests.")
		return
	}

	// Payload must conform to the following structure
	var currency Currency

	// Decode the request payload as an Request structure.	There will be an
	// error here if the incoming JSON is invalid
	err = json.Unmarshal(reqBody, &currency)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "request parameter is wrong"}})
		myLogger.Errorf("Error unmarshalling order request payload: %s", err)
		return
	}

	// 校验请求数据
	if len(currency.ID) <= 0 {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Currency cann't be empty"}})
		myLogger.Error("Currency cann't be empty.")
		return
	}
	if currency.Count < 0 {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Count must be greater than 0"}})
		myLogger.Error("Count must be greater than 0.")
		return
	}

	currency.User = enrollID
	// chaincode
	txid, err := releaseCurrency(currency.ID, int64(round(currency.Count, 6)*Multiple), currency.User)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "release Currency failed"}})
		myLogger.Errorf("release Currency failed:%s", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: txResult{Txid: txid}})

	myLogger.Debug("------------- Release Done")
}

// CheckRelease 检测发布币结果
// response说明：StatusBadRequest  失败  不需继续轮询，Error表示失败原因
//				StatusOK OK="1" 成功  不需继续轮询
//				StatusOK OK="0" 未果  需要继续轮询
func (a *AppREST) CheckRelease(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing check release request...")

	encoder := json.NewEncoder(rw)

	_, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("CheckRelease failed: [%s].", err)
		return
	}

	txid := req.PathParams["txid"]
	if txid == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Client must supply a id for checkrelease requests"}})
		myLogger.Errorf("Client must supply a id for checkrelease requests.")
		return
	}

	v, ok := chaincodeResult[txid]
	if !ok {
		rw.WriteHeader(http.StatusOK)
		encoder.Encode(restResp{Status: SUCCESS, Result: checkResult{Flag: "0"}})
	} else if v == Chaincode_Success {
		rw.WriteHeader(http.StatusOK)
		encoder.Encode(restResp{Status: SUCCESS, Result: checkResult{Flag: "1"}})
	} else {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: v}})
	}

	myLogger.Debug("------------- CheckRelease Done")
}

// Assign 分发币
func (a *AppREST) Assign(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing currency assign request...")

	encoder := json.NewEncoder(rw)

	enrollID, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("Assign failed: [%s].", err)
		return
	}

	// Read in the incoming request payload
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Internal JSON error when reading request body"}})
		myLogger.Error("Internal JSON error when reading request body.")
		return
	}
	myLogger.Debugf("Req body: %s", string(reqBody))
	// Incoming request body may not be empty, client must supply request payload
	if string(reqBody) == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Client must supply a payload for order requests"}})
		myLogger.Error("Client must supply a payload for order requests.")
		return
	}

	// Payload must conform to the following structure
	var assign struct {
		Currency string `json:"currency"`
		Assigns  []struct {
			Owner string `json:"owner"`
			Count int64  `json:"count"`
		} `json:"assigns"`
	}

	// Decode the request payload as an Request structure.	There will be an
	// error here if the incoming JSON is invalid
	err = json.Unmarshal(reqBody, &assign)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "request parameter is wrong"}})
		myLogger.Errorf("Error unmarshalling order request payload: %s", err)
		return
	}

	// 校验请求数据
	if len(assign.Currency) <= 0 {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Currency cann't be empty"}})
		myLogger.Error("Currency cann't be empty.")
		return
	}
	for k, v := range assign.Assigns {
		if v.Count < 0 {
			rw.WriteHeader(http.StatusBadRequest)
			encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Count must be greater than 0"}})
			myLogger.Error("Count must be greater than 0.")
			return
		}
		assign.Assigns[k].Count = int64(float64(v.Count) * Multiple)
	}

	assigns, _ := json.Marshal(&assign)
	// chaincode
	txid, err := assignCurrency(string(assigns), enrollID)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "assign Currency failed"}})
		myLogger.Errorf("assign Currency failed:%s", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: txResult{Txid: txid}})

	myLogger.Debug("------------- Assign Done")
}

// CheckAssign 检测分发币结果
// response说明：StatusBadRequest  失败  不需继续轮询，Error表示失败原因
//				StatusOK OK="1" 成功  不需继续轮询
//				StatusOK OK="0" 未果  需要继续轮询
func (a *AppREST) CheckAssign(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing check assign request...")

	encoder := json.NewEncoder(rw)

	_, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("CheckAssign failed: [%s].", err)
		return
	}

	txid := req.PathParams["txid"]
	if txid == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Client must supply a id for checkassign requests"}})
		myLogger.Errorf("Client must supply a id for checkassign requests.")
		return
	}

	v, ok := chaincodeResult[txid]
	if !ok {
		rw.WriteHeader(http.StatusOK)
		encoder.Encode(restResp{Status: SUCCESS, Result: checkResult{Flag: "0"}})
	} else if v == Chaincode_Success {
		rw.WriteHeader(http.StatusOK)
		encoder.Encode(restResp{Status: SUCCESS, Result: checkResult{Flag: "1"}})
	} else {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: v}})
	}

	myLogger.Debug("------------- CheckAssign Done")
}

// Exchange 挂单
func (a *AppREST) Exchange(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing order request...")

	encoder := json.NewEncoder(rw)

	enrollID, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("Exchange failed: [%s].", err)
		return
	}

	// Read in the incoming request payload
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Internal JSON error when reading request body"}})
		myLogger.Error("Internal JSON error when reading request body.")
		return
	}
	myLogger.Debugf("Req body: %s", string(reqBody))
	// Incoming request body may not be empty, client must supply request payload
	if string(reqBody) == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Client must supply a payload for order requests"}})
		myLogger.Error("Client must supply a payload for order requests.")
		return
	}

	// Payload must conform to the following structure
	var order Order

	// Decode the request payload as an Request structure.	There will be an
	// error here if the incoming JSON is invalid
	err = json.Unmarshal(reqBody, &order)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "request parameter format is wrong"}})
		myLogger.Errorf("Error unmarshalling order request payload: %s", err)
		return
	}

	// 校验请求数据
	if len(order.SrcCurrency) <= 0 {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "SrcCurrency cann't be empty"}})
		myLogger.Error("SrcCurrency cann't be empty.")
		return
	}
	if len(order.DesCurrency) <= 0 {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "DesCurrency cann't be empty"}})
		myLogger.Error("DesCurrency cann't be empty.")
		return
	}
	if order.SrcCount <= 0 {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "SrcCount must be greater than 0"}})
		myLogger.Error("SrcCount must be greater than 0.")
		return
	}
	if order.DesCount <= 0 {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "DesCount must be greater than 0"}})
		myLogger.Error("DesCount must be greater than 0.")
		return
	}

	//将挂单信息保存在待处理队列中
	uuid := util.GenerateUUID()
	order.Account = enrollID
	order.UUID = uuid
	order.RawUUID = uuid
	order.PendingTime = time.Now().Unix()
	order.PendingDate = time.Now().Format("2006-01-02 15:04:05")

	err = addOrder(uuid, &order)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "pending order failed"}})
		myLogger.Errorf("Error redis operation: %s", err)
		return
	}

	err = addSet(PendingOrdersKey, uuid)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "pending order failed"}})
		myLogger.Errorf("Error redis operation: %s", err)
		return
	}

	myLogger.Debugf("挂单信息: %+v", order)

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: txResult{Txid: uuid}})

	myLogger.Debug("------------- Exchange Done")
}

// CheckOrder  检测挂单结果，由前端轮询
// response说明：StatusBadRequest  挂单失败  不需继续轮询，Error表示失败原因
//				StatusOK OK="1"   挂单成功  不需继续轮询
//				StatusOK OK="0"   未果 需要继续轮询
func (a *AppREST) CheckOrder(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing check order request...")

	encoder := json.NewEncoder(rw)

	_, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("CheckOrder failed: [%s].", err)
		return
	}

	uuid := req.PathParams["uuid"]
	if uuid == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Client must supply a id for checkorder requests"}})
		myLogger.Errorf("Client must supply a id for checkorder requests.")
		return
	}

	// 1.检测该挂单是否在挂单成功队列中
	is, err := isInSet(PendSuccessOrdersKey, uuid)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "redis operation failed"}})
		myLogger.Errorf("Error redis operation: %s", err)
		return
	}
	if is {
		rw.WriteHeader(http.StatusOK)
		encoder.Encode(restResp{Status: SUCCESS, Result: checkResult{Flag: "1"}})
		myLogger.Debugf("%s 挂单成功", uuid)
		return
	}

	// 2.检测该挂单是否在挂单失败队列中
	is, err = isInSet(PendFailOrdersKey, uuid)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "redis operation failed"}})
		myLogger.Errorf("Error redis operation: %s", err)
		return
	}
	if is {
		order, err := getOrder(uuid)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "redis operation failed"}})
			myLogger.Errorf("Error redis operation: %s", err)
			return
		}

		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: order.Metadata}})
		myLogger.Debugf("%s 挂单失败", uuid)

		//如果检测到挂单失败，则将该挂单相关信息清除，因为失败的挂单相当于未保存到系统
		go clearFailedOrder(uuid)

		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: checkResult{Flag: "0"}})

	myLogger.Debug("------------- CheckOrder Done")
}

// Cancel 撤单
func (a *AppREST) Cancel(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing cancel order request...")

	encoder := json.NewEncoder(rw)

	_, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("Cancel failed: [%s].", err)
		return
	}

	// Read in the incoming request payload
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Internal JSON error when reading request body"}})
		myLogger.Error("Internal JSON error when reading request body.")
		return
	}
	myLogger.Debugf("Req body: %s", string(reqBody))

	var uuid struct {
		UUID string `json:"uuid"`
	}

	err = json.Unmarshal(reqBody, &uuid)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "request parameter format is wrong"}})
		myLogger.Errorf("Error unmarshalling order request payload: %s", err)
		return
	}

	// Incoming request body may not be empty, client must supply request payload
	if uuid.UUID == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Client must supply a payload for order requests"}})
		myLogger.Error("Client must supply a payload for order requests.")
		return
	}

	order, err := getOrder(uuid.UUID)

	// 在买卖队列中的（已锁定的）才有撤单
	key := getBSKey(order.SrcCurrency, order.DesCurrency)
	is := isInZSet(key, order.UUID)
	if is {
		// 1.将挂单从买入队列移到待撤单队列中
		err = mvBS2Cancel(key, order.UUID)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "redis operation failed"}})
			myLogger.Errorf("Error redis operation: %s", err)
			return
		}
	} else {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "Can't cancel order"}})
		myLogger.Errorf("Can't cancel order")
		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: txResult{Txid: uuid.UUID}})

	myLogger.Debug("------------- Cancel Done")
}

// CheckCancel 检查撤单是否成功
// response说明：StatusBadRequest撤单失败  不需继续轮询，Error表示失败原因
//				StatusOK OK="1" 撤单成功  不需继续轮询
//				StatusOK OK="0" 未果 需要继续轮询
func (a *AppREST) CheckCancel(rw web.ResponseWriter, req *web.Request) {
	myLogger.Info("REST processing check order cancel request...")

	encoder := json.NewEncoder(rw)

	_, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("CheckCancel failed: [%s].", err)
		return
	}

	uuid := req.PathParams["uuid"]
	if uuid == "" {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "Client must supply a id for checkorder requests"}})
		myLogger.Errorf("Client must supply a id for checkorder requests.")
		return
	}

	// 1.检测该挂单是否在撤单成功队列中
	is, err := isInSet(CancelSuccessOrderKey, uuid)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "redis operation failed"}})
		myLogger.Errorf("Error redis operation: %s", err)
		return
	}
	if is {
		rw.WriteHeader(http.StatusOK)
		encoder.Encode(restResp{Status: SUCCESS, Result: checkResult{Flag: "1"}})
		myLogger.Debugf("%s 撤单成功", uuid)

		return
	}

	// 2.检测该挂单是否在撤单失败队列中
	is, err = isInSet(CancelFailOrderKey, uuid)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "redis operation failed"}})
		myLogger.Errorf("Error redis operation: %s", err)
		return
	}
	if is {
		order, err := getOrder(uuid)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "redis operation failed"}})
			myLogger.Errorf("Error redis operation: %s", err)
			return
		}

		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: order.Metadata}})
		myLogger.Debugf("%s 撤单失败", uuid)
		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS, Result: checkResult{Flag: "0"}})

	myLogger.Debug("------------- CheckCancel Done")
}

// login confirms the account and secret password of the client with the
// CA and stores the enrollment certificate and key in the Devops server.
func (a *AppREST) Login(rw web.ResponseWriter, req *web.Request) {
	myLogger.Debug("------------- login...")

	encoder := json.NewEncoder(rw)

	// Decode the incoming JSON payload
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "request parameter is wrong"}})
		myLogger.Errorf("Failed login: [%s]", err)
		return
	}
	myLogger.Debugf("Req body: %s", string(reqBody))

	var loginRequest User
	err = json.Unmarshal(reqBody, &loginRequest)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "request parameter is wrong"}})
		myLogger.Errorf("Failed login: [%s]", err)
		return
	}

	// Check that the enrollId and enrollSecret are not left blank.
	if (loginRequest.EnrollID == "") || (loginRequest.EnrollSecret == "") {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: PARAMERR, Msg: "enrollId and enrollSecret can not be null"}})
		myLogger.Errorf("Failed login: [%s]", errors.New("enrollId and enrollSecret can not be null"))
		return
	}

	if connPeer == "grpc" {
		_, err = setCryptoClient(loginRequest.EnrollID, loginRequest.EnrollSecret)
	} else {
		err = loginRest(reqBody)
	}
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "username or pwd is wrong"}})
		myLogger.Errorf("Failed login: [%s]", err)
		return
	}

	// 初始化账户资产信息
	_, err = initAccount(loginRequest.EnrollID)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: SYSERR, Msg: "init account failed"}})
		myLogger.Errorf("Failed login: [%s]", err)
		return
	}

	http.SetCookie(rw, &http.Cookie{
		Name:   "loginfo",
		Value:  loginRequest.EnrollID,
		Path:   "/",
		MaxAge: 86400,
	})

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{
		Status: SUCCESS,
		Result: struct {
			UserInfo User `json:"userInfo"`
		}{
			UserInfo: User{EnrollID: loginRequest.EnrollID},
		}})
	myLogger.Debugf("Login successful for user '%s'.", loginRequest.EnrollID)

	myLogger.Debug("------------- login Done")
}

func (a *AppREST) Logout(rw web.ResponseWriter, req *web.Request) {
	myLogger.Debug("------------- logout...")

	encoder := json.NewEncoder(rw)

	// 删除cookie
	http.SetCookie(rw, &http.Cookie{
		Name:   "loginfo",
		Path:   "/",
		MaxAge: -1,
	})

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{Status: SUCCESS})

	myLogger.Debug("------------- logout Done")
}

func checkLogin(req *web.Request) (string, error) {
	cookie, err := req.Cookie("loginfo")
	if err != nil || cookie.Value == "" {
		return "", errors.New("not login")
	}

	return cookie.Value, nil
}

// IsLogin IsLogin
func (a *AppREST) IsLogin(rw web.ResponseWriter, req *web.Request) {
	myLogger.Debug("------------- islogin...")

	encoder := json.NewEncoder(rw)
	enrollID, err := checkLogin(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		encoder.Encode(restResp{Status: FAILED, Result: respErr{Code: NOTLOGIN, Msg: err.Error()}})
		myLogger.Errorf("IsLogin failed: [%s].", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{
		Status: SUCCESS,
		Result: struct {
			UserInfo User `json:"userInfo"`
		}{
			UserInfo: User{EnrollID: enrollID},
		}})

	myLogger.Debugf("IsLogin successful for user '%s'.", enrollID)

	myLogger.Debug("------------- islogin Done")
}

// Users Users
func (a *AppREST) Users(rw web.ResponseWriter, req *web.Request) {
	myLogger.Debug("------------- users...")
	encoder := json.NewEncoder(rw)

	rw.WriteHeader(http.StatusOK)
	encoder.Encode(restResp{
		Status: SUCCESS,
		Result: struct {
			Users []string `json:"users"`
		}{
			viper.GetStringSlice("app.users"),
		}})

	myLogger.Debug("------------- users Done")
}
