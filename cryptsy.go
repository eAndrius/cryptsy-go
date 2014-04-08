package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	API_PROTOCOL string = "https://"
	API_HOST     string = "api.cryptsy.com"
	API_PATH     string = "/api"
	API_IP       string // To be populated automatically
)

type Api struct {
	PublicKey  string
	PrivateKey string
	Conn       *http.Client
}

type Trade struct {
	Time     time.Time
	Price    float64
	Quantity float64
}

type Order struct {
	Price    float64
	Quantity float64
}

type Market struct {
	Label         string
	Volume        float64
	PrimaryName   string
	PrimaryCode   string
	SecondaryName string
	SecondaryCode string
	BuyFee        float64 // In normalized percentage
	SellFee       float64 // In normalized percentage
	MinOrder      float64
	RecentTrades  []Trade
	SellOrders    []Order
	BuyOrders     []Order

	MarketId int
}

type MarketKey struct {
	Primary   string
	Secondary string
	Reversed  bool
}

const (
	ACTION_SELL = "sell"
	ACTION_BUY  = "buy"
)

type ActionType string

type OrderAction struct {
	Market   MarketKey
	Action   ActionType
	Price    float64
	Quantity float64
}

type Balance struct {
	Name    string
	Balance float64
}

type BalanceKey string

func New(publicKey, privateKey string) (api *Api, err error) {
	// Resolve api host to IP to avoid time-consuming DNS queries
	api_ip, err := net.LookupHost(API_HOST)
	if err != nil {
		return nil, err
	}
	API_IP = api_ip[0]
	//API_HOST = API_IP

	// Create HTTP pool
	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, //, MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS10},
		MaxIdleConnsPerHost:   5,
		DisableKeepAlives:     false,
		DisableCompression:    false,
		ResponseHeaderTimeout: 5 * time.Second,
	}

	api = &Api{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Conn:       &http.Client{Transport: tr},
	}

	return api, err
}

func (api *Api) query(params url.Values) (result string, err error) {
	// Add nonce to the request
	params.Set("nonce", strconv.Itoa(int(time.Now().UnixNano())))
	//params.Set("cachebuster", strconv.Itoa(int(time.Now().UnixNano())))

	// Generate HMAC based on parameters
	mac := hmac.New(sha512.New, []byte(api.PrivateKey))
	mac.Write([]byte(params.Encode()))
	sign := hex.EncodeToString(mac.Sum(nil))

	// Construct query
	r, _ := http.NewRequest("POST", API_PROTOCOL+API_HOST+API_PATH, bytes.NewBufferString(params.Encode()))
	r.Header.Set("Key", api.PublicKey)
	r.Header.Set("Sign", sign)
	r.Header.Set("Connection", "Keep-Alive")
	//r.Header.Set("User-Agent", "Mozilla/5.0 (compatible; CRYPTSY-API/1.0; MSIE 6.0 compatible; +cryptsybot@motoko.sutas.eu)")
	r.Header.Set("Cache-Control", "no-cache, must-revalidate")
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Content-Length", strconv.Itoa(len(params.Encode())))
	//r.Close = true

	resp, err := api.Conn.Do(r)
	if err != nil {
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close() //  This allows the connection to be reused
	//r.Body.Close()
	//resp.Close = true
	if err != nil {
		return
	}

	return string(body), nil
}

func (api *Api) getInfo() (balances map[BalanceKey]Balance, err error) {
	resultJson, err := api.query(url.Values{"method": {"getinfo"}})
	if err != nil {
		return nil, err
	}

	type resp struct {
		Success, Error string
		Return         map[string](map[string]string)
	}
	var tmp resp

	json.Unmarshal([]byte(resultJson), &tmp)

	if tmp.Success != "1" {
		return nil, errors.New(tmp.Error)
	}

	balances = make(map[BalanceKey]Balance, len(tmp.Return["balances_available"]))

	for key, value := range tmp.Return["balances_available"] {
		balance, _ := strconv.ParseFloat(value, 64)
		balances[BalanceKey(key)] = Balance{Name: key, Balance: balance}
	}

	return balances, nil
}

func (api *Api) getMarkets() (markets map[MarketKey]Market, err error) {
	resultJson, err := api.query(url.Values{"method": {"getmarkets"}})
	if err != nil {
		return nil, err
	}

	type resp struct {
		Success, Error string
		Return         [](map[string]string)
	}
	var tmp resp

	json.Unmarshal([]byte(resultJson), &tmp)

	if tmp.Success != "1" {
		return nil, errors.New(tmp.Error)
	}

	markets = make(map[MarketKey]Market)
	for _, market := range tmp.Return {
		key := MarketKey{
			Primary:   market["primary_currency_code"],
			Secondary: market["secondary_currency_code"],
			Reversed:  false,
		}

		tmpid, _ := strconv.Atoi(market["marketid"])
		value := Market{
			Label:         market["label"],
			PrimaryName:   market["primary_currency_name"],
			PrimaryCode:   market["primary_currency_code"],
			SecondaryName: market["secondary_currency_name"],
			SecondaryCode: market["secondary_currency_code"],
			BuyFee:        0.002, //float64 // In normalized percentage
			SellFee:       0.003, //float64 // In normalized percentage
			MarketId:      tmpid, //int
			//Volume        market[""],//float64
		}

		markets[key] = value
	}

	return
}

/*
func (api *Api) getMyTransactions() (result string, err error) {
	result, err = api.query(url.Values{"method": {"mytransactions"}})
	return
}

func (api *Api) getMarketTrades(marketId int) (result string, err error) {
	result, err = api.query(url.Values{"method": {"markettrades"}, "marketid": {strconv.Itoa(marketId)}})
	return
}

// See: getDepth
func (api *Api) getMarketOrders(marketId int) (result string, err error) {
	result, err = api.query(url.Values{"method": {"marketorders"}, "marketid": {strconv.Itoa(marketId)}})
	return
}

func (api *Api) getMyTrades(marketId, limit int) (result string, err error) {
	result, err = api.query(url.Values{"method": {"mytrades"}, "marketid": {strconv.Itoa(marketId)}, "limit": {strconv.Itoa(limit)}})
	return
}

func (api *Api) getAllMyTrades() (result string, err error) {
	result, err = api.query(url.Values{"method": {"allmytrades"}})
	return
}
*/

// TODO: myorders, , cancelmarketorders, calculatefees, generatenewaddress

func (api *Api) getAllMyOrders() (orderIds map[int]bool, err error) {
	resultJson, err := api.query(url.Values{"method": {"allmyorders"}})
	if err != nil {
		return
	}

	type resp struct {
		Success, Error string
		Return         []map[string]interface{}
	}
	var tmp resp

	json.Unmarshal([]byte(resultJson), &tmp)

	if tmp.Success != "1" {
		return nil, errors.New(tmp.Error)
	}

	orderIds = make(map[int]bool)
	for _, value := range tmp.Return {
		tmpi, _ := strconv.Atoi(value["orderid"].(string))
		orderIds[tmpi] = true
	}

	return
}

func (api *Api) getDepth(marketId int) (sellOrders, buyOrders []Order, err error) {
	resultJson, err := api.query(url.Values{"method": {"depth"}, "marketid": {strconv.Itoa(marketId)}})
	if err != nil {
		return
	}

	type resp struct {
		Success, Error string
		Return         map[string]interface{}
	}
	var tmp resp

	json.Unmarshal([]byte(resultJson), &tmp)

	if tmp.Success != "1" {
		return nil, nil, errors.New(tmp.Error)
	}

	for key, value := range tmp.Return {
		for _, order := range value.([]interface{}) {
			price, _ := strconv.ParseFloat(order.([]interface{})[0].(string), 64)
			quantity, _ := strconv.ParseFloat(order.([]interface{})[1].(string), 64)

			if key == "buy" {
				buyOrders = append(buyOrders, Order{Price: price, Quantity: quantity})
			} else if key == "sell" {
				sellOrders = append(sellOrders, Order{Price: price, Quantity: quantity})
			}
		}
	}

	return
}

func (api *Api) createOrder(marketId int, ordertype ActionType, quantity, price float64) (orderId int, err error) {
	resultJson, err := api.query(url.Values{"method": {"createorder"}, "marketid": {strconv.Itoa(marketId)}, "ordertype": {string(ordertype)},
		"quantity": {strconv.FormatFloat(quantity, 'f', 8, 64)}, "price": {strconv.FormatFloat(price, 'f', 8, 64)}})

	if err != nil {
		return
	}

	type resp struct {
		Success, OrderId string
		Error            string
	}
	var tmp resp

	json.Unmarshal([]byte(resultJson), &tmp)

	if tmp.Success != "1" {
		return 0, errors.New(tmp.Error)
	}

	return strconv.Atoi(tmp.OrderId)
}

func (api *Api) cancelOrder(orderId int) (err error) {
	resultJson, err := api.query(url.Values{"method": {"cancelorder"}, "orderid": {strconv.Itoa(orderId)}})
	if err != nil {
		return
	}
	//fmt.Println(resultJson)
	type resp struct {
		Success, Return, Error string
	}
	var tmp resp

	json.Unmarshal([]byte(resultJson), &tmp)

	if tmp.Success != "1" {
		return errors.New(tmp.Error)
	}

	return nil
}

func (api *Api) cancelAllOrders() (err error) {
	resultJson, err := api.query(url.Values{"method": {"cancelallorders"}})
	if err != nil {
		return
	}

	type resp struct {
		Success, Error string
	}
	var tmp resp

	json.Unmarshal([]byte(resultJson), &tmp)

	if tmp.Success != "1" {
		return errors.New(tmp.Error)
	}

	return nil
}
