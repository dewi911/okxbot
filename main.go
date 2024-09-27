package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	apiKey     = "YOUR_API_KEY"
	secretKey  = "YOUR_SECRET_KEY"
	passphrase = "YOUR_PASSPHRASE"
	baseURL    = "https://www.okx.com"
)

type TickerResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		InstId string `json:"instId"`
		Last   string `json:"last"`
	} `json:"data"`
}

type OrderRequest struct {
	InstId  string `json:"instId"`
	TdMode  string `json:"tdMode"`
	Side    string `json:"side"`
	OrdType string `json:"ordType"`
	Sz      string `json:"sz"`
	Px      string `json:"px,omitempty"`
}

type OrderResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		OrdId   string `json:"ordId"`
		ClOrdId string `json:"clOrdId"`
		Tag     string `json:"tag"`
		SCode   string `json:"sCode"`
		SMsg    string `json:"sMsg"`
	} `json:"data"`
}

func fetchPrice(instId string) (float64, error) {
	url := fmt.Sprintf("%s/api/v5/market/ticker?instId=%s", baseURL, instId)
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("ошибка при выполнении запроса: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("ошибка при чтении ответа: %v", err)
	}

	var tickerResp TickerResponse
	err = json.Unmarshal(body, &tickerResp)
	if err != nil {
		return 0, fmt.Errorf("ошибка при разборе JSON: %v", err)
	}

	if len(tickerResp.Data) > 0 {
		return strconv.ParseFloat(tickerResp.Data[0].Last, 64)
	}
	return 0, fmt.Errorf("данные о цене не найдены")
}

func sign(timestamp, method, requestPath string, body []byte) string {
	message := timestamp + method + requestPath + string(body)
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func placeOrder(instId string, side string, size float64, price float64) error {
	endpoint := "/api/v5/trade/order"
	url := baseURL + endpoint

	orderReq := OrderRequest{
		InstId:  instId,
		TdMode:  "cash",
		Side:    side,
		OrdType: "limit",
		Sz:      fmt.Sprintf("%.8f", size),
		Px:      fmt.Sprintf("%.8f", price),
	}

	body, err := json.Marshal(orderReq)
	if err != nil {
		return fmt.Errorf("ошибка при маршалинге запроса: %v", err)
	}

	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	signature := sign(timestamp, "POST", endpoint, body)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("ошибка при создании запроса: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OK-ACCESS-KEY", apiKey)
	req.Header.Set("OK-ACCESS-SIGN", signature)
	req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("OK-ACCESS-PASSPHRASE", passphrase)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка при выполнении запроса: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка при чтении ответа: %v", err)
	}

	var orderResp OrderResponse
	err = json.Unmarshal(respBody, &orderResp)
	if err != nil {
		return fmt.Errorf("ошибка при разборе ответа: %v", err)
	}

	if orderResp.Code != "0" {
		return fmt.Errorf("ошибка при размещении ордера: %s", orderResp.Msg)
	}

	fmt.Printf("Ордер успешно размещен. ID ордера: %s\n", orderResp.Data[0].OrdId)
	return nil
}

func monitorAndTrade(instId string, interval time.Duration, targetPrice float64, tradeSize float64) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		price, err := fetchPrice(instId)
		if err != nil {
			fmt.Println("Ошибка при получении цены:", err)
			continue
		}

		fmt.Printf("Текущая цена %s: %.8f\n", instId, price)

		if price <= targetPrice {
			fmt.Printf("Цена достигла или опустилась ниже целевой: %.8f. Выполняем покупку.\n", price)
			err := placeOrder(instId, "buy", tradeSize, price)
			if err != nil {
				fmt.Println("Ошибка при размещении ордера:", err)
			} else {
				fmt.Printf("Покупка выполнена успешно. Размер: %.8f, Цена: %.8f\n", tradeSize, price)
				return
			}
		}
	}
}

func main() {
	instId := "ZETA-USDT"
	interval := 1 * time.Second
	targetPrice := 0.5
	tradeSize := 100.0

	fmt.Printf("Начало мониторинга цены %s...\n", instId)
	monitorAndTrade(instId, interval, targetPrice, tradeSize)
}
