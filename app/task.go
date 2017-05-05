package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type LockInfo struct {
	Owner    string `json:"owner"`
	Currency string `json:"currency"`
	OrderId  string `json:"orderId"`
	Count    int64  `json:"count"`
}

// lockBalance 锁定挂单余额
func lockBalance() {
	batch := viper.GetInt64("redis.batch.pending")
	sleep := viper.GetDuration("app.task.polling.lock")

	for {
		// 1.取出待挂单
		uuids, err := getBathSetMember(PendingOrdersKey, batch)
		if err != nil || len(uuids) == 0 {
			continue
		}

		myLogger.Debugf("锁定挂单 %s 余额...", uuids)

		// 2.调用chaincode锁定相关信息
		lockInfo := getLockInfo(uuids)
		lock(lockInfo, true, "lock")

		time.Sleep(sleep * time.Second)
	}
}

// lockSuccess 锁定成功
func lockSuccess(uuids []string) {
	for _, uuid := range uuids {
		// 1.修改挂单完成时间
		updateOrderTime(uuid, time.Now().Unix(), 0)

		// 2.将挂单放到买卖队列,并放到账户对应的挂单集合中
		mvPending2BS(uuid)
	}
}

// lockFail 锁定失败
func lockFail(fails []FailInfo) {
	for _, v := range fails {
		// 1.保存失败信息
		saveOrderMetadata(v.Id, v.Info)
		// 2.将之从待挂单队列移动到挂单失败队列
		mvPending2Failed(v.Id)
	}
}

// matchTx 撮合交易
func matchTx() {
	sleep := viper.GetDuration("app.task.polling.match")

	for {
		// 买卖队列所有key
		keys, _ := getKeys(ExchangeKey + "*")
		keyMap := make(map[string]string, 0)

		for _, key := range keys {
			if _, ok := keyMap[key]; ok {
				continue
			}

			// 1.取买卖队列中的第一个挂单
			buyUUID, err := getFirstZSet(key)
			if err != nil || len(buyUUID) == 0 {
				continue
			}
			// 2.校验挂单是否过期
			buyOrder, isExpired := checkExpired(buyUUID)
			if isExpired {
				continue
			}

			keyMap[key] = key
			key = getBSKeyByOne(key)
			keyMap[key] = key

			// 3.取买卖出队列中对应的第一个挂单
			sellUUID, err := getFirstZSet(key)
			if err != nil || len(sellUUID) == 0 {
				continue
			}
			// 4.校验是否过期
			sellOrder, isExpired := checkExpired(sellUUID)
			if isExpired {
				continue
			}

			myLogger.Debugf("%s 的卖出价：%f/%f=%.6f %s", buyOrder.SrcCurrency, buyOrder.DesCount, buyOrder.SrcCount, buyOrder.DesCount/buyOrder.SrcCount, buyOrder.DesCurrency)
			myLogger.Debugf("%s 的买入价：%f/%f=%.6f %s", sellOrder.DesCurrency, sellOrder.SrcCount, sellOrder.DesCount, sellOrder.SrcCount/sellOrder.DesCount, sellOrder.SrcCurrency)
			// 5.比较价格，进行撮合
			if buyOrder.DesCount/buyOrder.SrcCount > sellOrder.SrcCount/sellOrder.DesCount {
				continue
			}

			myLogger.Debugf("匹配成功，买入挂单：%s, 卖出挂单：%s", buyUUID, sellUUID)

			// 6.撮合成功，处理买卖挂单
			dealMatchOrder(buyOrder, sellOrder, time.Now().Unix())
		}
		time.Sleep(sleep * time.Second)
	}
}

type ExchangeOrder struct {
	BuyOrder  *OrderInt `json:"buyOrder"`
	SellOrder *OrderInt `json:"sellOrder"`
}

// execTx 执行撮合交易
func execTx() {
	batch := viper.GetInt64("redis.batch.matched")
	sleep := viper.GetDuration("app.task.polling.exec")

	for {
		// 1.取出撮合好的一对交易
		uuids, err := getBathSetMember(MatchedOrdersKey, batch)
		if err != nil || len(uuids) == 0 {
			continue
		}

		myLogger.Debugf("执行撮合好的交易 %s ...", uuids)

		// 2.chaincode执行交易
		exchanges := []*ExchangeOrder{}
		for _, v := range uuids {
			towUuid := strings.Split(v, ",")

			buyOrder, err := getOrder(towUuid[0])
			if err != nil {
				continue
			}
			sellOrder, err := getOrder(towUuid[1])
			if err != nil {
				continue
			}

			buyOrderInt := &OrderInt{
				UUID:         buyOrder.UUID,
				Account:      buyOrder.Account,
				SrcCurrency:  buyOrder.SrcCurrency,
				SrcCount:     int64(buyOrder.SrcCount * Multiple),
				DesCurrency:  buyOrder.DesCurrency,
				DesCount:     int64(buyOrder.DesCount * Multiple),
				IsBuyAll:     buyOrder.IsBuyAll,
				ExpiredTime:  buyOrder.ExpiredTime,
				PendingTime:  buyOrder.PendingTime,
				PendedTime:   buyOrder.PendedTime,
				MatchedTime:  buyOrder.MatchedTime,
				FinishedTime: buyOrder.FinishedTime,
				RawUUID:      buyOrder.RawUUID,
				Metadata:     buyOrder.Metadata,
				FinalCost:    int64(buyOrder.FinalCost * Multiple),
			}

			sellOrderInt := &OrderInt{
				UUID:         sellOrder.UUID,
				Account:      sellOrder.Account,
				SrcCurrency:  sellOrder.SrcCurrency,
				SrcCount:     int64(sellOrder.SrcCount * Multiple),
				DesCurrency:  sellOrder.DesCurrency,
				DesCount:     int64(sellOrder.DesCount * Multiple),
				IsBuyAll:     sellOrder.IsBuyAll,
				ExpiredTime:  sellOrder.ExpiredTime,
				PendingTime:  sellOrder.PendingTime,
				PendedTime:   sellOrder.PendedTime,
				MatchedTime:  sellOrder.MatchedTime,
				FinishedTime: sellOrder.FinishedTime,
				RawUUID:      sellOrder.RawUUID,
				Metadata:     sellOrder.Metadata,
				FinalCost:    int64(sellOrder.FinalCost * Multiple),
			}
			exchangeOrder := &ExchangeOrder{BuyOrder: buyOrderInt, SellOrder: sellOrderInt}
			exchanges = append(exchanges, exchangeOrder)
		}
		exchangeStr, _ := json.Marshal(&exchanges)

		exchange(string(exchangeStr))

		time.Sleep(sleep * time.Second)
	}
}

// execTxSuccess 执行交易成功
func execTxSuccess(uuids []string) {
	for _, v := range uuids {
		// 1.从撮合好队列移动到交易成功队列，并修改交易完成时间和status=1
		mvExec2Success(MatchedOrdersKey, v)
	}
}

// execTxFail 执行交易失败
func execTxFail(fails []FailInfo) {
	// 暂无处理
}

// dealExpired 处理过期挂单
func dealExpired(uuids ...string) {
	for _, uuid := range uuids {
		// 1.从买卖队列移到过期队列中,且status=2
		mvBS2Expired(uuid)
	}
}

// execExpired 处理过期挂单
func execExpired(uuid ...string) {
	batch := viper.GetInt64("redis.batch.expired")
	sleep := viper.GetDuration("app.task.polling.expired")

	for {
		// 1.从过期队列中取出一个
		uuids, err := getBathSetMember(ExpiredOrdersKey, batch)
		if err != nil || len(uuids) == 0 {
			continue
		}

		myLogger.Debugf("处理过期挂单 %s ...", uuids)

		// 2.chaincode处理过期交易
		lockInfo := getLockInfo(uuids)
		lock(lockInfo, false, "expire")

		time.Sleep(sleep * time.Second)
	}
}

// expiredSuccess 处理过期挂单成功
func expiredSuccess(uuids []string) {
	// 1.从过期队列移到过期成功队列
	for _, v := range uuids {
		mvExpired2Success(v)
	}
}

// expiredFail 处理过期挂单失败
func expiredFail(fails []FailInfo) {
	//暂无处理
}

// findExpired 定时任务查找过期挂单
func findExpired() {
	sleep := viper.GetDuration("app.task.polling.findexpired")

	for {
		// 取出买卖队列中所有挂单进行判断
		uuidsBuy, _ := getAllBS()
		for _, v := range uuidsBuy {
			checkExpired(v)
		}

		time.Sleep(sleep * time.Second)
	}
}

// checkExpired 校验过期
func checkExpired(uuid string) (*Order, bool) {
	order, err := getOrder(uuid)
	if err != nil {
		return nil, true
	}

	if order.ExpiredTime > 0 && order.ExpiredTime <= time.Now().Unix() {
		dealExpired(uuid)
		myLogger.Debugf("挂单 %s 已过期", uuid)

		return nil, true
	}
	return order, false
}

// execCancel 撤单
func execCancel() {
	batch := viper.GetInt64("redis.batch.cancel")
	sleep := viper.GetDuration("app.task.polling.cancel")

	for {
		// 1.从撤单队列中取出一个
		uuids, err := getBathSetMember(CancelingOrderKey, batch)
		if err != nil || len(uuids) == 0 {
			continue
		}

		myLogger.Debugf("处理撤销挂单 %s ...", uuids)

		// 2.chaincode处理撤销交易
		lockInfo := getLockInfo(uuids)
		lock(lockInfo, false, "cancel")

		time.Sleep(sleep * time.Second)
	}
}

// cancelSuccess 撤单成功
func cancelSuccess(uuids []string) {
	// 1.从待撤单队列移到撤单成功队列,且status=3
	for _, v := range uuids {
		mvCancle2Success(v)
	}
}

// cancelFailed 撤单失败
func cancelFailed(fails []FailInfo) {
	for _, v := range fails {
		// 1.保存失败信息
		saveOrderMetadata(v.Id, v.Info)
		// 2.将挂单放回买卖队列，并保存撤单失败信息
		mvCancel2BS(v.Id)
	}
}

func getLockInfo(uuids []string) string {
	locks := []*LockInfo{}
	for _, v := range uuids {
		order, err := getOrder(v)
		if err != nil {
			continue
		}
		lockinfo := LockInfo{
			Owner:    order.Account,
			Currency: order.SrcCurrency,
			OrderId:  order.UUID,
			Count:    int64(order.SrcCount * Multiple),
		}

		locks = append(locks, &lockinfo)
	}

	lockInfos, _ := json.Marshal(&locks)
	return string(lockInfos)
}
