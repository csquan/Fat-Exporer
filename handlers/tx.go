package handlers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"eth2-exporter/types"
	"eth2-exporter/utils"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	geth_types "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gorilla/mux"
)

var txTemplate = template.Must(template.New("tx").Funcs(utils.GetTemplateFuncs()).ParseFiles("templates/layout.html", "templates/tx.html"))
var txNotFoundTemplate = template.Must(template.New("txnotfound").Funcs(utils.GetTemplateFuncs()).ParseFiles("templates/layout.html", "templates/txnotfound.html"))

// Tx will show the tx using a go template
func Tx(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	vars := mux.Vars(r)
	txHashString := strings.Replace(vars["txHash"], "0x", "", -1)

	data := InitPageData(w, r, "txs", "/tx", "Transaction")
	data.HeaderAd = true

	txHash, err := hex.DecodeString(strings.ReplaceAll(txHashString, "0x", ""))

	if err != nil {
		data.Meta.Title = fmt.Sprintf("%v - Transaction %v - beaconcha.in - %v", utils.Config.Frontend.SiteName, txHashString, time.Now().Year())
		data.Meta.Path = "/tx/" + txHashString
		logger.Errorf("error parsing tx hash %v: %v", txHashString, err)
		err = txNotFoundTemplate.ExecuteTemplate(w, "layout", data)

		if err != nil {
			logger.Errorf("error executing template for %v route: %v", r.URL.String(), err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		return
	}

	data.Meta.Title = fmt.Sprintf("%v - Tx 0x%x - beaconcha.in - %v", utils.Config.Frontend.SiteName, txHash, time.Now().Year())
	data.Meta.Path = fmt.Sprintf("/tx/0x%x", txHash)

	client, err := ethclient.Dial(utils.Config.Frontend.Eth1Endpoint)
	if err != nil {
		logger.Errorf("error initializing ethclient for route %v: %v", r.URL.String(), err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	tx, pending, err := client.TransactionByHash(context.Background(), common.BytesToHash(txHash))

	if err != nil {
		logger.Errorf("error retrieving data for tx %v for route %v: %v", txHashString, r.URL.String(), err)
		err = txNotFoundTemplate.ExecuteTemplate(w, "layout", data)

		if err != nil {
			logger.Errorf("error executing template for %v route: %v", r.URL.String(), err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		return
	}

	receipt, err := client.TransactionReceipt(context.Background(), common.BytesToHash(txHash))
	if err != nil {
		logger.Errorf("error retrieving receipt data for tx %v for route %v: %v", txHashString, r.URL.String(), err)
		err = txNotFoundTemplate.ExecuteTemplate(w, "layout", data)

		if err != nil {
			logger.Errorf("error executing template for %v route: %v", r.URL.String(), err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		return
	}

	code, err := client.CodeAt(context.Background(), *tx.To(), nil)
	if err != nil {
		logger.Errorf("error retrieving to code data for tx %v for route %v: %v", txHashString, r.URL.String(), err)
		err = txNotFoundTemplate.ExecuteTemplate(w, "layout", data)

		if err != nil {
			logger.Errorf("error executing template for %v route: %v", r.URL.String(), err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		return
	}

	header, err := client.HeaderByHash(context.Background(), receipt.BlockHash)
	if err != nil {
		logger.Errorf("error retrieving block fror tx %v for route %v: %v", txHashString, r.URL.String(), err)
		err = txNotFoundTemplate.ExecuteTemplate(w, "layout", data)

		if err != nil {
			logger.Errorf("error executing template for %v route: %v", r.URL.String(), err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		return
	}

	msg, err := tx.AsMessage(geth_types.NewLondonSigner(tx.ChainId()), header.BaseFee)
	if err != nil {
		logger.Errorf("error converting tx to msg %v for route %v: %v", txHashString, r.URL.String(), err)
		err = txNotFoundTemplate.ExecuteTemplate(w, "layout", data)

		if err != nil {
			logger.Errorf("error executing template for %v route: %v", r.URL.String(), err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		return
	}
	txPageData := types.TxPageData{
		GethTx:           tx,
		IsPending:        pending,
		Receipt:          receipt,
		TargetIsContract: len(code) != 0,
		From:             msg.From(),
		Header:           header,
		TxFee:            new(big.Int).Mul(tx.GasPrice(), new(big.Int).SetUint64(receipt.GasUsed)),
	}

	data.Data = txPageData

	if utils.IsApiRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(data.Data)
	} else {
		err = txTemplate.ExecuteTemplate(w, "layout", data)
	}

	if err != nil {
		logger.Errorf("error executing template for %v route: %v", r.URL.String(), err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
