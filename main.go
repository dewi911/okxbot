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

type CandleResponse struct {
	Code string     `json:"code"`
	Msg  string     `json:"msg"`
	Data [][]string `json:"data"`
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

func fetchCandles(instId string, period string, limit int) ([][]float64, error) {
	url := fmt.Sprintf("%s/api/v5/market/candles?instId=%s&bar=%s&limit=%d", baseURL, instId, period, limit)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ошибка при выполнении запроса: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка при чтении ответа: %v", err)
	}

	var candleResp CandleResponse
	err = json.Unmarshal(body, &candleResp)
	if err != nil {
		return nil, fmt.Errorf("ошибка при разборе JSON: %v", err)
	}

	candles := make([][]float64, 0, len(candleResp.Data))
	for _, candle := range candleResp.Data {
		if len(candle) < 5 {
			continue
		}
		timestamp, _ := strconv.ParseFloat(candle[0], 64)
		open, _ := strconv.ParseFloat(candle[1], 64)
		high, _ := strconv.ParseFloat(candle[2], 64)
		low, _ := strconv.ParseFloat(candle[3], 64)
		close, _ := strconv.ParseFloat(candle[4], 64)
		candles = append(candles, []float64{timestamp, open, high, low, close})
	}

	return candles, nil
}

func calculateMA(candles [][]float64, period int) float64 {
	if len(candles) < period {
		return 0
	}
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += candles[i][4]
	}
	return sum / float64(period)
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

func tradingBot(instId string, interval time.Duration, buyThreshold, sellThreshold, tradeSize float64) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var position float64 = 0

	for range ticker.C {
		price, err := fetchPrice(instId)
		if err != nil {
			fmt.Println("Ошибка при получении цены:", err)
			continue
		}

		candles, err := fetchCandles(instId, "5m", 50)
		if err != nil {
			fmt.Println("Ошибка при получении свечей:", err)
			continue
		}

		ma20 := calculateMA(candles, 20)
		ma50 := calculateMA(candles, 50)

		fmt.Printf("Текущая цена %s: %.8f, MA20: %.8f, MA50: %.8f\n", instId, price, ma20, ma50)

		if position == 0 && ma20 > ma50 && price <= buyThreshold {
			fmt.Printf("MA20 пересекла MA50 сверху и цена ниже порога покупки. Выполняем покупку.\n")
			err := placeOrder(instId, "buy", tradeSize/price, price)
			if err != nil {
				fmt.Println("Ошибка при размещении ордера на покупку:", err)
			} else {
				position = tradeSize / price
				fmt.Printf("Покупка выполнена успешно. Размер: %.8f %s\n", position, instId)
			}
		} else if position > 0 && (ma20 < ma50 || price >= sellThreshold) {
			fmt.Printf("MA20 пересекла MA50 снизу или цена выше порога продажи. Выполняем продажу.\n")
			err := placeOrder(instId, "sell", position, price)
			if err != nil {
				fmt.Println("Ошибка при размещении ордера на продажу:", err)
			} else {
				fmt.Printf("Продажа выполнена успешно. Размер: %.8f %s\n", position, instId)
				position = 0
			}
		}
	}
}

func main() {
	instId := "ZETA-USDT"
	interval := 1 * time.Second
	buyThreshold := 0.7
	sellThreshold := 0.80
	tradeSize := 100.0

	fmt.Printf("Запуск торгового бота для %s...\n", instId)
	tradingBot(instId, interval, buyThreshold, sellThreshold, tradeSize)
}
