package main

import (
	"bytes"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
)

func convertInteger2Decimal(num int64) float64 {
	nStr := strconv.FormatInt(num, 10)

	return float64(num) / math.Pow10(len(nStr))
}

func getScore(srcCount, desCount float64, time int64) float64 {
	return round(desCount/srcCount, 6)*math.Pow10(6) + convertInteger2Decimal(time)
}

func convertPriceReciprocal(num int64) float64 {
	return math.Pow10(10) / float64(num)
}

func getBSKey(srcCurrency, desCurrency string) string {
	return ExchangeKey + "_" + srcCurrency + "_" + desCurrency
}

func getBSKeyByOne(key string) string {
	splits := strings.Split(key, "_")

	return getBSKey(splits[2], splits[1])
}

// round 四舍五入
func round(val float64, places int) float64 {
	var t float64
	f := math.Pow10(places)
	x := val * f
	if math.IsInf(x, 0) || math.IsNaN(x) {
		return val
	}
	if x >= 0.0 {
		t = math.Ceil(x)
		if (t - x) > 0.50000000001 {
			t -= 1.0
		}
	} else {
		t = math.Ceil(-x)
		if (t + x) > 0.50000000001 {
			t -= 1.0
		}
		t = -t
	}
	x = t / f

	if !math.IsInf(x, 0) {
		return x
	}

	return t
}

// 对挂单按时间排序
type Orders []*Order

func (x Orders) Len() int           { return len(x) }
func (x Orders) Less(i, j int) bool { return x[i].PendedTime > x[j].PendedTime }
func (x Orders) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

type Historys []*History

func (x Historys) Len() int           { return len(x) }
func (x Historys) Less(i, j int) bool { return x[i].Time > x[j].Time }
func (x Historys) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

func doHTTPPost(url string, reqBody []byte) ([]byte, error) {
	resp, err := http.Post(url, "application/json;charset=utf-8", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
