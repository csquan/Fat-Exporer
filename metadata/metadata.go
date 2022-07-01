package metadata

import (
	"context"
	"eth2-exporter/types"
	"eth2-exporter/utils"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	geth_types "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	lru "github.com/hashicorp/golang-lru"
	"github.com/sirupsen/logrus"
)

var client *ethclient.Client
var cache *lru.Cache

func Init() error {
	c, err := ethclient.Dial(utils.Config.Frontend.Metadata.Eth1Endpoint)

	if err != nil {
		return err
	}

	cache, err = lru.New(1024)
	if err != nil {
		return err
	}

	client = c
	return nil
}

func GetEth1Transaction(hash common.Hash) (*types.Eth1TxData, error) {
	cacheKey := fmt.Sprintf("tx:%s", hash.String())
	cached, found := cache.Get(cacheKey)

	if found {
		logrus.Infof("retrieved data for tx %v from cache", hash)
		return cached.(*types.Eth1TxData), nil
	}

	tx, pending, err := client.TransactionByHash(context.Background(), hash)

	if err != nil {
		return nil, fmt.Errorf("error retrieving data for tx %v: %v", hash, err)
	}

	if pending {
		return nil, fmt.Errorf("error retrieving data for tx %v: tx is still pending", hash)
	}

	receipt, err := GetTransactionReceipt(hash)
	if err != nil {
		return nil, fmt.Errorf("error retrieving receipt data for tx %v: %v", hash, err)
	}

	code, err := GetCodeAt(*tx.To())
	if err != nil {
		return nil, fmt.Errorf("error retrieving code data for tx %v receipient %v: %v", hash, tx.To(), err)
	}

	header, err := GetBlockHeaderByHash(receipt.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("error retrieving block header data for tx %v: %v", hash, err)
	}

	msg, err := tx.AsMessage(geth_types.NewLondonSigner(tx.ChainId()), header.BaseFee)
	if err != nil {
		return nil, fmt.Errorf("error converting tx %v to message: %v", hash, err)
	}
	txPageData := &types.Eth1TxData{
		GethTx:           tx,
		IsPending:        pending,
		Receipt:          receipt,
		TargetIsContract: len(code) != 0,
		From:             msg.From(),
		Header:           header,
		TxFee:            new(big.Int).Mul(tx.GasPrice(), new(big.Int).SetUint64(receipt.GasUsed)),
	}

	cache.Add(cacheKey, txPageData)

	return txPageData, nil
}

func GetCodeAt(address common.Address) ([]byte, error) {
	cacheKey := fmt.Sprintf("a:%s", address.String())
	cached, found := cache.Get(cacheKey)

	if found {
		logrus.Infof("retrieved code data for address %v from cache", address)
		return cached.([]byte), nil
	}

	code, err := client.CodeAt(context.Background(), address, nil)
	if err != nil {
		return nil, fmt.Errorf("error retrieving code data for address %v: %v", address, err)
	}

	cache.Add(cacheKey, code)

	return code, nil
}

func GetBlockHeaderByHash(hash common.Hash) (*geth_types.Header, error) {
	cacheKey := fmt.Sprintf("h:%s", hash.String())
	cached, found := cache.Get(cacheKey)

	if found {
		logrus.Infof("retrieved header data for block %v from cache", hash)
		return cached.(*geth_types.Header), nil
	}

	header, err := client.HeaderByHash(context.Background(), hash)
	if err != nil {
		return nil, fmt.Errorf("error retrieving block header data for tx %v: %v", hash, err)
	}

	cache.Add(cacheKey, header)

	return header, nil
}

func GetTransactionReceipt(hash common.Hash) (*geth_types.Receipt, error) {
	cacheKey := fmt.Sprintf("r:%s", hash.String())
	cached, found := cache.Get(cacheKey)

	if found {
		logrus.Infof("retrieved receipt data for tx %v from cache", hash)
		return cached.(*geth_types.Receipt), nil
	}

	receipt, err := client.TransactionReceipt(context.Background(), hash)
	if err != nil {
		return nil, fmt.Errorf("error retrieving receipt data for tx %v: %v", hash, err)
	}

	cache.Add(cacheKey, receipt)

	return receipt, nil
}
