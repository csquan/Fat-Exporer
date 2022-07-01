package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	geth_types "github.com/ethereum/go-ethereum/core/types"
)

// TxPageData is a struct to hold detailed tx data for the tx page
type Eth1TxData struct {
	From             common.Address
	GethTx           *geth_types.Transaction
	Receipt          *geth_types.Receipt
	Header           *geth_types.Header
	IsPending        bool
	TargetIsContract bool
	TxFee            *big.Int
}
