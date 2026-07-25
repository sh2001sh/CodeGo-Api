package domain

import (
	"encoding/json"
	"log"
	"sync"
)

var topupGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}

var topupGroupRatioMutex sync.RWMutex

func TopupGroupRatioJSON() string {
	topupGroupRatioMutex.RLock()
	defer topupGroupRatioMutex.RUnlock()

	jsonBytes, err := json.Marshal(topupGroupRatio)
	if err != nil {
		log.Printf("error marshalling topup group ratio: %s", err.Error())
	}
	return string(jsonBytes)
}

func UpdateTopupGroupRatio(jsonStr string) error {
	topupGroupRatioMutex.Lock()
	defer topupGroupRatioMutex.Unlock()

	topupGroupRatio = make(map[string]float64)
	return json.Unmarshal([]byte(jsonStr), &topupGroupRatio)
}

func GetTopupGroupRatio(name string) float64 {
	topupGroupRatioMutex.RLock()
	defer topupGroupRatioMutex.RUnlock()

	ratio, ok := topupGroupRatio[name]
	if !ok {
		log.Printf("topup group ratio not found: %s", name)
		return 1
	}
	return ratio
}
