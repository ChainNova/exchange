package main

import (
	"encoding/json"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/hyperledger/fabric/core/util"
	"github.com/spf13/viper"
	"gopkg.in/redis.v5"
)

const (
	PendingOrdersKey       = "pendingOrders"       //待挂单队列
	PendSuccessOrdersKey   = "pendSuccessOrders"   //挂单失败队列
	PendFailOrdersKey      = "pendFailOrders"      //挂单失败队列
	ExchangeKey            = "exchange"            //交易队列  exchange_[srcCurrency]_[desCurrency] 格式
	ExchangeSuccessKey     = "exchangeSuccess"     //交易执行成功队列
	LastPriceKey           = "lastPrice"           //上次成交价
	MatchedOrdersKey       = "matchedOrders"       //撮合的交易等待chaincode处理
	ExpiredOrdersKey       = "expiredOrders"       //过期挂单队列
	ExpiredSuccessOrderKey = "expiredSuccessOrder" //过期处理成功
	CancelingOrderKey      = "cancelingOrders"     //待撤销挂单
	CancelSuccessOrderKey  = "cancelSuccessOrders" //撤销挂单成功
	CancelFailOrderKey     = "cancelFailOrders"    //撤销挂单失败

)

var client *redis.Client

func initRedis() {
	addr := viper.GetString("redis.address")
	pwd := viper.GetString("redis.pwd")
	db := viper.GetInt("redis.db")
	// poolsize := viper.GetInt("redis.poolsize")

	client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pwd,
		DB:       db,
		// PoolSize: poolsize,
	})

	if _, err := client.Ping().Result(); err != nil {
		myLogger.Errorf("Connection redis [%s] failed: %s", addr, err)
		os.Exit(-1)
	} else {
		myLogger.Debugf("Connection redis [%s] successed.", addr)
	}
}

func addOrder(key string, value *Order) error {
	listValue, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return client.Set(key, string(listValue), 0).Err()
}

func getOrder(uuid string) (*Order, error) {
	js, err := getString(uuid)
	if err != nil {
		return nil, err
	}

	var order Order
	err = json.Unmarshal([]byte(js), &order)
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func getOrderByUser(user string) ([]*Order, error) {
	// 用户挂单集中于挂单成功队列和以用户名为key的队列中 且 不在以下三种状态的 status = 0
	// 完成的挂单存在于交易执行成功队列中 status = 1
	// 过期的挂单存在于过期成功队列中 status = 2
	// 撤单的挂单存在于撤单成功队列中 status = 3
	uuids, err := getAllSetMember("user_" + user)
	if err != nil || len(uuids) == 0 {
		return nil, err
	}

	txs := []*Order{}

	for _, v := range uuids {
		order, _ := getOrder(v)

		txs = append(txs, order)
	}
	// 对order排序

	// golang1.8 的方法
	// sort.Slice(txs, func(i, j int) bool { return txs[i].PendedTime > txs[j].PendedTime })
	sort.Sort(Orders(txs))

	return txs, nil
}

func saveOrderMetadata(uuid, msg string) error {
	order, err := getOrder(uuid)
	if err != nil {
		return err
	}

	order.Metadata = msg

	err = addOrder(uuid, order)
	if err != nil {
		return err
	}

	return nil
}

// updateTime 更新挂单完成时间和交易完成时间
func updateOrderTime(uuid string, PendedTime, FinishedTime int64) error {
	order, err := getOrder(uuid)
	if err != nil {
		return err
	}

	if PendedTime > 0 {
		order.PendedTime = PendedTime
		order.PendedDate = time.Unix(PendedTime, 0).Format("2006-01-02 15:04:05")
	}
	if FinishedTime > 0 {
		order.FinishedTime = FinishedTime
		order.FinishedDate = time.Unix(FinishedTime, 0).Format("2006-01-02 15:04:05")
	}

	err = addOrder(uuid, order)
	if err != nil {
		return err
	}
	return nil
}

func getString(key string) (string, error) {
	return client.Get(key).Result()
}

func getLastPrice() int64 {
	price, _ := client.Get(LastPriceKey).Int64()

	return price
}

func setLastPrice(price int64) error {
	return client.Set(LastPriceKey, price, 0).Err()
}

func isKeyExists(key string) (bool, error) {
	return client.Exists(key).Result()
}

func getKeys(pattern string) ([]string, error) {
	return client.Keys(pattern).Result()
}

func addSet(key, value string) error {
	return client.SAdd(key, value).Err()
}

func getBathSetMember(key string, count int64) ([]string, error) {
	return client.SRandMemberN(key, count).Result()
}

func getAllSetMember(key string) ([]string, error) {
	return client.SMembers(key).Result()
}

func popSet(key string) (string, error) {
	return client.SPop(key).Result()
}

func isInSet(key, value string) (bool, error) {
	return client.SIsMember(key, value).Result()
}

func getFirstZSet(key string) (string, error) {
	cmd := client.ZRange(key, 0, 0)

	if len(cmd.Val()) > 0 {
		return cmd.Val()[0], cmd.Err()
	}
	return "", cmd.Err()
}

func isInZSet(key, member string) bool {
	if err := client.ZRank(key, member).Err(); err != nil {
		return false
	}

	return true
}

func mvPending2BS(uuid string) error {
	order, err := getOrder(uuid)
	if err != nil {
		return err
	}

	// ******************************************
	// *******将挂单移到买卖队列，确保事务性**********
	// ******************************************
	pipe := client.Pipeline()
	// multi := client.Multi()

	//从待挂单队列中移除
	pipe.SRem(PendingOrdersKey, uuid)
	//添加到买卖队列
	member := redis.Z{Member: uuid}

	key := getBSKey(order.SrcCurrency, order.DesCurrency)

	// x个币A->y个币B 存入ZSet的score为y/x，相当于A的卖出价格，B的买入价格即为y/x
	// X个币B->Y个币A 存入ZSet的score为Y/X，相当于B的卖出价格，A的买入价格即为X/Y
	// 这样，两个都按从小到大排序，那么恰好就是卖出按价格从小到大，买入价格从大到小
	member.Score = getScore(order.SrcCount, order.DesCount, order.PendingTime)
	pipe.ZAdd(key, member)

	// 添加到挂单成功队列
	pipe.SAdd(PendSuccessOrdersKey, uuid)

	// 添加到账户挂单队列
	pipe.SAdd("user_"+order.Account, uuid)

	_, err = pipe.Exec()

	return err
}

func temp() error {
	return nil
}

func mvPending2Failed(uuid string) error {
	return client.SMove(PendingOrdersKey, PendFailOrdersKey, uuid).Err()
}

func dealMatchOrder(buyOrder, sellOrder *Order, timeStamp int64) error {
	// ***********************注意**********************
	// ******买单的源币目标币正好与卖单的源币目标币相反********
	// ******只要撮合成功，则必定不会出现锁定余额不足的情况********
	// ************************************************

	// 成交价=目标币（buyOrder中的）数量/源币（buyOrder中的）数量  以撮合成的订单中较早挂单的价格为准
	buyPrice := buyOrder.DesCount / buyOrder.SrcCount
	sellPrice := sellOrder.SrcCount / sellOrder.DesCount
	endPrice := buyPrice
	if buyOrder.PendingTime > sellOrder.PendingTime {
		endPrice = sellPrice
	}

	// 交易数以buyOrder的目标币为单位
	endCount := float64(0)
	if buyOrder.IsBuyAll && sellOrder.IsBuyAll {
		// 两个挂单都是买完为止，则是否分单要以对方所能提供币数为准，即两单以同一币为单位取小者为交易数
		endCount = math.Min(buyOrder.DesCount, sellOrder.DesCount*endPrice)
	} else if !buyOrder.IsBuyAll && !sellOrder.IsBuyAll {
		// 两个挂单都是卖完为止，则是否分单要以对方所能接收币数为准，即两单以同一币为单位取小者为交易数
		endCount = math.Min(buyOrder.SrcCount*endPrice, sellOrder.SrcCount)
	} else if buyOrder.IsBuyAll && !sellOrder.IsBuyAll {
		// 两个挂单一个是买完一个是卖完，则是否分单要以买尽挂单的目标币数和卖尽挂单的源币数为准，即买尽挂单的目标币数和卖尽挂单的源币数取小者为交易数
		endCount = math.Min(buyOrder.DesCount, sellOrder.SrcCount)
	} else if !buyOrder.IsBuyAll && sellOrder.IsBuyAll {
		// 两个挂单一个是买完一个是卖完，则是否分单要以买尽挂单的目标币数和卖尽挂单的源币数为准，即买尽挂单的目标币数和卖尽挂单的源币数取小者为交易数
		endCount = math.Min(buyOrder.SrcCount, sellOrder.DesCount) * endPrice
	}

	// ******************************************
	// *******将处理挂单撮合，确保事务性**********
	// ******************************************
	pipe := client.Pipeline()

	//匹配的成对UUID，“买入挂单UUID,卖出挂单UUID”
	matchBuyUUID := buyOrder.UUID
	matchSellUUID := sellOrder.UUID
	matchUUID := ""

	// 1.将完成的挂单从队列中移除,并修改撮合时间
	// 2.将未完成的挂单剩余部分修改对应key的交易数量
	if !buyOrder.IsBuyAll && buyOrder.SrcCount*endPrice > endCount {
		// 卖完为止时，源币有剩余则生成新单
		tempBuyOrder := Order{}
		tempBuyOrder.UUID = util.GenerateUUID()
		tempBuyOrder.SrcCount = endCount / endPrice
		tempBuyOrder.DesCount = endCount
		tempBuyOrder.MatchedTime = timeStamp
		tempBuyOrder.MatchedDate = time.Unix(timeStamp, 0).Format("2006-01-02 15:04:05")
		tempBuyOrder.FinalCost = tempBuyOrder.SrcCount

		tempBuyOrder.Account = buyOrder.Account
		tempBuyOrder.SrcCurrency = buyOrder.SrcCurrency
		tempBuyOrder.DesCurrency = buyOrder.DesCurrency
		tempBuyOrder.IsBuyAll = buyOrder.IsBuyAll
		tempBuyOrder.ExpiredTime = buyOrder.ExpiredTime
		tempBuyOrder.ExpiredDate = buyOrder.ExpiredDate
		tempBuyOrder.PendingTime = buyOrder.PendingTime
		tempBuyOrder.PendingDate = buyOrder.PendingDate
		tempBuyOrder.PendedTime = buyOrder.PendedTime
		tempBuyOrder.PendedDate = buyOrder.PendedDate
		tempBuyOrder.RawUUID = buyOrder.UUID
		tempBuyOrder.RawSrcCount = buyOrder.SrcCount
		tempBuyOrder.RawDesCount = buyOrder.DesCount

		buyOrder.SrcCount = buyOrder.SrcCount - endCount/endPrice
		buyOrder.DesCount = buyOrder.SrcCount * buyPrice

		js, _ := json.Marshal(buyOrder)
		pipe.Set(buyOrder.UUID, string(js), 0)

		js, _ = json.Marshal(&tempBuyOrder)
		pipe.Set(tempBuyOrder.UUID, string(js), 0)
		pipe.SAdd("user_"+tempBuyOrder.Account, tempBuyOrder.UUID)

		matchBuyUUID = tempBuyOrder.UUID
	} else if buyOrder.IsBuyAll && buyOrder.DesCount > endCount {
		// 买完为止时，目标币有剩余则生成新单
		tempBuyOrder := Order{}
		tempBuyOrder.UUID = util.GenerateUUID()
		tempBuyOrder.SrcCount = endCount / endPrice
		tempBuyOrder.DesCount = endCount
		tempBuyOrder.MatchedTime = timeStamp
		tempBuyOrder.MatchedDate = time.Unix(timeStamp, 0).Format("2006-01-02 15:04:05")
		tempBuyOrder.FinalCost = tempBuyOrder.SrcCount

		tempBuyOrder.Account = buyOrder.Account
		tempBuyOrder.SrcCurrency = buyOrder.SrcCurrency
		tempBuyOrder.DesCurrency = buyOrder.DesCurrency
		tempBuyOrder.IsBuyAll = buyOrder.IsBuyAll
		tempBuyOrder.ExpiredTime = buyOrder.ExpiredTime
		tempBuyOrder.ExpiredDate = buyOrder.ExpiredDate
		tempBuyOrder.PendingTime = buyOrder.PendingTime
		tempBuyOrder.PendingDate = buyOrder.PendingDate
		tempBuyOrder.PendedTime = buyOrder.PendedTime
		tempBuyOrder.PendedDate = buyOrder.PendedDate
		tempBuyOrder.RawUUID = buyOrder.UUID
		tempBuyOrder.RawSrcCount = buyOrder.SrcCount
		tempBuyOrder.RawDesCount = buyOrder.DesCount

		buyOrder.DesCount = buyOrder.DesCount - endCount
		buyOrder.SrcCount = buyOrder.DesCount / buyPrice

		js, _ := json.Marshal(buyOrder)
		pipe.Set(buyOrder.UUID, string(js), 0)

		js, _ = json.Marshal(&tempBuyOrder)
		pipe.Set(tempBuyOrder.UUID, string(js), 0)
		pipe.SAdd("user_"+tempBuyOrder.Account, tempBuyOrder.UUID)

		matchBuyUUID = tempBuyOrder.UUID
	} else {
		pipe.ZRem(getBSKey(buyOrder.SrcCurrency, buyOrder.DesCurrency), buyOrder.UUID)

		buyOrder.FinalCost = endCount / endPrice
		buyOrder.MatchedTime = timeStamp
		buyOrder.MatchedDate = time.Unix(timeStamp, 0).Format("2006-01-02 15:04:05")

		js, _ := json.Marshal(buyOrder)
		pipe.Set(buyOrder.UUID, string(js), 0)

		matchBuyUUID = buyOrder.UUID
	}

	if !sellOrder.IsBuyAll && sellOrder.SrcCount > endCount {
		// 卖完为止时，源币有剩余则生成新单
		tempSellOrder := Order{}
		tempSellOrder.UUID = util.GenerateUUID()
		tempSellOrder.SrcCount = endCount
		tempSellOrder.DesCount = endCount / endPrice
		tempSellOrder.MatchedTime = timeStamp
		tempSellOrder.MatchedDate = time.Unix(timeStamp, 0).Format("2006-01-02 15:04:05")
		tempSellOrder.FinalCost = tempSellOrder.SrcCount

		tempSellOrder.Account = sellOrder.Account
		tempSellOrder.SrcCurrency = sellOrder.SrcCurrency
		tempSellOrder.DesCurrency = sellOrder.DesCurrency
		tempSellOrder.IsBuyAll = sellOrder.IsBuyAll
		tempSellOrder.ExpiredTime = sellOrder.ExpiredTime
		tempSellOrder.ExpiredDate = sellOrder.ExpiredDate
		tempSellOrder.PendingTime = sellOrder.PendingTime
		tempSellOrder.PendingDate = sellOrder.PendingDate
		tempSellOrder.PendedTime = sellOrder.PendedTime
		tempSellOrder.PendedDate = sellOrder.PendedDate
		tempSellOrder.RawUUID = sellOrder.UUID
		tempSellOrder.RawSrcCount = sellOrder.SrcCount
		tempSellOrder.RawDesCount = sellOrder.DesCount

		sellOrder.SrcCount = sellOrder.SrcCount - endCount
		sellOrder.DesCount = sellOrder.SrcCount / sellPrice

		js, _ := json.Marshal(sellOrder)
		pipe.Set(sellOrder.UUID, string(js), 0)

		js, _ = json.Marshal(&tempSellOrder)
		pipe.Set(tempSellOrder.UUID, string(js), 0)
		pipe.SAdd("user_"+tempSellOrder.Account, tempSellOrder.UUID)

		matchSellUUID = tempSellOrder.UUID
	} else if sellOrder.IsBuyAll && sellOrder.DesCount > endCount/endPrice {
		// 买完为止时，目标币有剩余则生成新单
		tempSellOrder := *sellOrder
		tempSellOrder.UUID = util.GenerateUUID()
		tempSellOrder.SrcCount = endCount
		tempSellOrder.DesCount = endCount / endPrice
		tempSellOrder.MatchedTime = timeStamp
		tempSellOrder.MatchedDate = time.Unix(timeStamp, 0).Format("2006-01-02 15:04:05")
		tempSellOrder.FinalCost = tempSellOrder.SrcCount

		tempSellOrder.Account = sellOrder.Account
		tempSellOrder.SrcCurrency = sellOrder.SrcCurrency
		tempSellOrder.DesCurrency = sellOrder.DesCurrency
		tempSellOrder.IsBuyAll = sellOrder.IsBuyAll
		tempSellOrder.ExpiredTime = sellOrder.ExpiredTime
		tempSellOrder.ExpiredDate = sellOrder.ExpiredDate
		tempSellOrder.PendingTime = sellOrder.PendingTime
		tempSellOrder.PendingDate = sellOrder.PendingDate
		tempSellOrder.PendedTime = sellOrder.PendedTime
		tempSellOrder.PendedDate = sellOrder.PendedDate
		tempSellOrder.RawUUID = sellOrder.UUID
		tempSellOrder.RawSrcCount = sellOrder.SrcCount
		tempSellOrder.RawDesCount = sellOrder.DesCount

		sellOrder.DesCount = sellOrder.DesCount - endCount/endPrice
		sellOrder.SrcCount = sellOrder.DesCount * sellPrice

		js, _ := json.Marshal(sellOrder)
		pipe.Set(sellOrder.UUID, string(js), 0)

		js, _ = json.Marshal(&tempSellOrder)
		pipe.Set(tempSellOrder.UUID, string(js), 0)
		pipe.SAdd("user_"+tempSellOrder.Account, tempSellOrder.UUID)

		matchSellUUID = tempSellOrder.UUID
	} else {
		pipe.ZRem(getBSKey(sellOrder.SrcCurrency, sellOrder.DesCurrency), sellOrder.UUID)

		sellOrder.FinalCost = endCount
		sellOrder.MatchedTime = timeStamp
		sellOrder.MatchedDate = time.Unix(timeStamp, 0).Format("2006-01-02 15:04:05")

		js, _ := json.Marshal(sellOrder)
		pipe.Set(sellOrder.UUID, string(js), 0)

		matchSellUUID = sellOrder.UUID
	}

	matchUUID = matchBuyUUID + "," + matchSellUUID

	// 3.将撮合成功的两个挂单放到别处等待chaincode处理
	// 部分交易的挂单要赋予新的uuid，以免跟剩余部分的uuid重复
	pipe.SAdd(MatchedOrdersKey, matchUUID)

	_, err := pipe.Exec()

	return err
}

func getBSKeyByUUID(uuid string) string {
	order, _ := getOrder(uuid)
	return getBSKey(order.SrcCurrency, order.DesCurrency)
}

func mvBS2Expired(uuid string) error {
	pipe := client.Pipeline()

	pipe.SAdd(ExpiredOrdersKey, uuid)
	pipe.ZRem(getBSKeyByUUID(uuid), uuid)

	order, err := getOrder(uuid)
	if err != nil {
		return err
	}

	order.Status = 2

	listValue, err := json.Marshal(order)
	if err != nil {
		return err
	}
	pipe.Set(uuid, string(listValue), 0)

	_, err = pipe.Exec()

	return err
}

func rmSetMember(key, member string) error {
	return client.SRem(key, member).Err()
}

func clearFailedOrder(uuid string) {
	client.Del(uuid)
	rmSetMember(PendingOrdersKey, uuid)
	rmSetMember(PendFailOrdersKey, uuid)
}

func mvBS2Cancel(bsKey, uuid string) error {
	pipe := client.Pipeline()

	pipe.ZRem(bsKey, uuid)
	pipe.SAdd(CancelingOrderKey, uuid)

	_, err := pipe.Exec()

	return err
}

func mvCancel2BS(uuid string) error {
	order, err := getOrder(uuid)
	if err != nil {
		return err
	}
	key := getBSKey(order.SrcCurrency, order.DesCurrency)
	// ******************************************
	// *******将撤单失败的还原回买卖队列，确保事务性**********
	// ******************************************
	pipe := client.Pipeline()

	//从待撤单队列中移除
	pipe.SRem(PendingOrdersKey, uuid)
	//还原到买卖队列
	member := redis.Z{Member: uuid}
	member.Score = order.DesCount / order.SrcCount
	pipe.ZAdd(key, member)

	// 添加到挂单失败队列
	pipe.SAdd(CancelFailOrderKey, uuid)

	_, err = pipe.Exec()

	return err
}

func mvCancle2Success(uuid string) error {
	pipe := client.Pipeline()

	pipe.SMove(CancelingOrderKey, CancelSuccessOrderKey, uuid)

	order, err := getOrder(uuid)
	if err != nil {
		return err
	}

	order.Status = 3

	listValue, err := json.Marshal(order)
	if err != nil {
		return err
	}
	pipe.Set(uuid, string(listValue), 0)

	_, err = pipe.Exec()

	return err
}

func mvExpired2Success(uuid string) error {
	return client.SMove(ExpiredOrdersKey, ExpiredSuccessOrderKey, uuid).Err()
}

func mvExec2Success(key, uuid string) error {
	pipe := client.Pipeline()

	pipe.SRem(key, uuid)
	uuid2 := strings.Split(uuid, ",")
	pipe.SAdd(ExchangeSuccessKey, uuid2[0])
	pipe.SAdd(ExchangeSuccessKey, uuid2[1])

	order, err := getOrder(uuid2[0])
	if err != nil {
		return err
	}
	order.Status = 1
	order.MatchedUUID = uuid2[1]
	order.FinishedTime = time.Now().Unix()
	order.FinishedDate = time.Unix(order.FinishedTime, 0).Format("2006-01-02 15:04:05")

	listValue, err := json.Marshal(order)
	if err != nil {
		return err
	}
	pipe.Set(uuid2[0], string(listValue), 0)

	order, err = getOrder(uuid2[1])
	if err != nil {
		return err
	}
	order.Status = 1
	order.MatchedUUID = uuid2[0]
	order.FinishedTime = time.Now().Unix()
	order.FinishedDate = time.Unix(order.FinishedTime, 0).Format("2006-01-02 15:04:05")

	listValue, err = json.Marshal(order)
	if err != nil {
		return err
	}
	pipe.Set(uuid2[1], string(listValue), 0)

	_, err = pipe.Exec()

	return err
}

func getAllBS() ([]string, error) {
	// 所有以ExchangeKey开头的key
	keys, err := getKeys(ExchangeKey + "*")
	if err != nil {
		return nil, err
	}

	uuids := []string{}
	for _, v := range keys {
		count := client.ZCard(v).Val()

		copy(uuids, client.ZRange(v, 0, count).Val())
	}
	return uuids, nil
}

func getRangeZSet(key string, count int64) ([]string, error) {
	return client.ZRange(key, 0, count-1).Result()
}
