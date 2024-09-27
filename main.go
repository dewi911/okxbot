package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

type TickerResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		InstId string `json:"instId"`
		Last   string `json:"last"`
	} `json:"data"`
}

func fetchPrice(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("ошибка при выполнении запроса: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка при чтении ответа: %v", err)
	}

	var tickerResp TickerResponse
	err = json.Unmarshal(body, &tickerResp)
	if err != nil {
		return "", fmt.Errorf("ошибка при разборе JSON: %v", err)
	}

	if len(tickerResp.Data) > 0 {
		return tickerResp.Data[0].Last, nil
	}
	return "", fmt.Errorf("данные о цене не найдены")
}

func monitorPrice(url string, interval time.Duration, targetPrice float64) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		price, err := fetchPrice(url)
		if err != nil {
			fmt.Println("Ошибка:", err)
			continue
		}

		fmt.Printf("Текущая цена ZETA/USDT: %s\n", price)

		currentPrice, err := strconv.ParseFloat(price, 64)
		if err != nil {
			fmt.Println("Ошибка при преобразовании цены:", err)
			continue
		}

		if currentPrice <= targetPrice {
			fmt.Printf("Цена достигла или опустилась ниже целевой: %.4f\n", currentPrice)

		}
	}
}

func main() {
	url := "https://www.okx.com/api/v5/market/ticker?instId=ZETA-USDT"
	interval := 2 * time.Second
	targetPrice := 0.5

	fmt.Println("Начало мониторинга цены ZETA/USDT...")
	monitorPrice(url, interval, targetPrice)
}
