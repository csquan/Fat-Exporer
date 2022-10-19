package handlers

import (
	"encoding/json"
	"eth2-exporter/db"
	"eth2-exporter/templates"
	"eth2-exporter/types"
	"eth2-exporter/utils"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"time"
)

const (
	visibleDigitsForHash         = 8
	minimumTransactionsPerUpdate = 25
)

func Eth1Transactions(w http.ResponseWriter, r *http.Request) {

	var eth1TransactionsTemplate = templates.GetTemplate("layout.html", "execution/transactions.html")

	w.Header().Set("Content-Type", "text/html")

	data := InitPageData(w, r, "blockchain", "/eth1transactions", "Transactions")
	data.Data = getTransactionDataStartingWithPageToken("")

	err := eth1TransactionsTemplate.ExecuteTemplate(w, "layout", data)
	if err != nil {
		logger.Errorf("error executing template for %v route: %v", r.URL.String(), err)
		http.Error(w, "Internal server error", http.StatusServiceUnavailable)
	}
}

func Eth1TransactionsData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(getTransactionDataStartingWithPageToken(r.URL.Query().Get("pageToken")))
	if err != nil {
		logger.Errorf("error enconding json response for %v route: %v", r.URL.String(), err)
		http.Error(w, "Internal server error", http.StatusServiceUnavailable)
	}
}

func getTransactionDataStartingWithPageToken(pageToken string) *types.DataTableResponse {
	pageTokenId := uint64(1)
	//{
	//	if len(pageToken) > 0 {
	//		v, err := strconv.ParseUint(pageToken, 10, 64)
	//		if err == nil && v > 0 {
	//			pageTokenId = v
	//		}
	//	}
	//}
	//if pageTokenId == 0 {
	//	pageTokenId = services.LatestEth1BlockNumber()
	//}

	//tableData := make([][]interface{}, 0, minimumTransactionsPerUpdate)--改完后前段不显示全部交易--loading

	t, err := GetTransactions()
	if err != nil {

	}
	tableData := make([][]interface{}, 0, len(t))
	for _, v := range t {
		method := "Transfer"
		//{
		//	d := v.GetData()
		//	if len(d) > 3 {
		//		m := d[:4]
		//
		//		if len(v.GetItx()) > 0 || v.GetGasUsed() > 21000 || v.GetErrorMsg() != "" { // check for invokesContract
		//			method = fmt.Sprintf("0x%x", m)
		//		} else {
		//			method = "Transfer*"
		//		}
		//	}
		//}

		var toText template.HTML
		{
			//to := v.GetTo()
			//if len(to) > 0 {
			//	toText = utils.FormatAddressWithLimits(to, names[string(v.GetTo())], false, "address", visibleDigitsForHash+5, 18, true)
			//} else {
			//	itx := v.GetItx()
			//	if len(itx) > 0 && itx[0] != nil {
			//		to = itx[0].GetTo()
			//		if len(to) > 0 {
			//			toText = utils.FormatAddressWithLimits(to, "Contract Creation", true, "address", visibleDigitsForHash+5, 18, true)
			//		}
			//	}
			//}
		}

		tableData = append(tableData, []interface{}{
			utils.FormatAddressWithLimits(v.GetHash(), "", false, "tx", visibleDigitsForHash+5, 18, true),
			utils.FormatMethod(method),
			template.HTML(fmt.Sprintf(`<A href="block/%d">%v</A>`, 10, utils.FormatAddCommas(10))),
			utils.FormatTimestamp(time.Now().Unix()),
			//utils.FormatAddressWithLimits(v.GetFrom(), names[string(v.GetFrom())], false, "address", visibleDigitsForHash+5, 18, true),
			toText,
			utils.FormatAmountFormated(new(big.Int).SetBytes(v.GetValue()), "ETH", 8, 4, true, true, false),
			utils.FormatAmountFormated(db.CalculateTxFeeFromTransaction(v, new(big.Int).SetUint64(0)), "ETH", 8, 4, true, true, false),
		})
	}

	return &types.DataTableResponse{
		Data:        tableData,
		PagingToken: fmt.Sprintf("%d", pageTokenId),
	}
}

func GetTransactions() ([]*types.Eth1Transaction, error) {
	txs, err := db.GetTransactions()
	if err != nil {

	}
	return txs, nil
}
