package main

import (
	"encoding/json"
	"os"

	"github.com/spf13/viper"
	"gopkg.in/redis.v5"
)

var client *redis.Client

const (
	ChaincodeResultKey      = "chaincodeResult"      // 存储chaincode执行结果，key为txid，value为结果，最终成功以blockEvent为准
	ChaincodeBatchResultKey = "chaincodeBatchResult" // 存储chaincode批量操作结果，key为txid，value为结果。该内容与chaincodeResult结合确定最终执行结果
)

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

func getChaincodeID() (string, error) {
	chaincodeKey := viper.GetString("event.chaincode.key")

	return client.Get(chaincodeKey).Result()
}

func isKeyExists(key string) (bool, error) {
	return client.Exists(key).Result()
}

func setChaincodeResult(k, v string) error {
	key := ChaincodeResultKey + "_" + k
	// 如果已经存在，说明收到过其他节点反馈的该事件，不再保存
	if is, err := isKeyExists(key); is || err != nil {
		return err
	}

	pipe := client.Pipeline()

	pipe.Set(key, v, 0)
	pipe.SAdd(ChaincodeResultKey, k)

	_, err := pipe.Exec()

	return err
}

func setChaincodeBatchResult(k string, r BatchResult) error {
	key := ChaincodeBatchResultKey + "_" + k
	// 如果已经存在，说明收到过其他节点反馈的该事件，不再保存
	if is, err := isKeyExists(key); is || err != nil {
		return err
	}

	v, err := json.Marshal(r)
	if err != nil {
		return err
	}

	return client.Set(key, string(v), 0).Err()
}
