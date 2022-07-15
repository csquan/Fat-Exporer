package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"eth2-exporter/types"
	"eth2-exporter/utils"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
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

	txPageData := &types.Eth1TxData{
		GethTx:    tx,
		IsPending: pending,
		Events:    make([]*types.Eth1EventData, 0, 10),
	}

	receipt, err := GetTransactionReceipt(hash)
	if err != nil {
		return nil, fmt.Errorf("error retrieving receipt data for tx %v: %v", hash, err)
	}
	txPageData.Receipt = receipt
	txPageData.TxFee = new(big.Int).Mul(tx.GasPrice(), new(big.Int).SetUint64(receipt.GasUsed))

	code, err := GetCodeAt(*tx.To())
	if err != nil {
		return nil, fmt.Errorf("error retrieving code data for tx %v receipient %v: %v", hash, tx.To(), err)
	}
	txPageData.TargetIsContract = len(code) != 0

	header, err := GetBlockHeaderByHash(receipt.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("error retrieving block header data for tx %v: %v", hash, err)
	}
	txPageData.Header = header

	msg, err := tx.AsMessage(geth_types.NewLondonSigner(tx.ChainId()), header.BaseFee)
	if err != nil {
		return nil, fmt.Errorf("error converting tx %v to message: %v", hash, err)
	}
	txPageData.From = msg.From()

	if len(receipt.Logs) > 0 {
		for _, log := range receipt.Logs {
			contractAbi, name, err := GetABIForContract(log.Address)

			if err != nil {
				logrus.Errorf("error retrieving abi for contract %v: %v", tx.To(), err)
				eth1Event := &types.Eth1EventData{
					Address:     log.Address,
					Name:        "",
					Topics:      log.Topics,
					Data:        log.Data,
					DecodedData: map[string]string{},
				}

				txPageData.Events = append(txPageData.Events, eth1Event)
			} else {
				txPageData.ToName = name
				boundContract := bind.NewBoundContract(*tx.To(), *contractAbi, nil, nil, nil)

				for name, event := range contractAbi.Events {
					if bytes.Equal(event.ID.Bytes(), log.Topics[0].Bytes()) {
						logData := make(map[string]interface{})
						err := boundContract.UnpackLogIntoMap(logData, name, *log)

						if err != nil {
							logrus.Errorf("error decoding event %v", name)
						}

						eth1Event := &types.Eth1EventData{
							Address:     log.Address,
							Name:        strings.Replace(event.String(), "event ", "", 1),
							Topics:      log.Topics,
							Data:        log.Data,
							DecodedData: map[string]string{},
						}

						for name, val := range logData {
							eth1Event.DecodedData[name] = fmt.Sprintf("0x%x", val)
						}

						txPageData.Events = append(txPageData.Events, eth1Event)
					}

				}
			}
		}

		//

		// for _, log := range receipt.Logs {
		// 	var unpackedLog interface{}
		// 	boundContract.UnpackLog(unpackedLog, )
		// }
	}

	// cache.Add(cacheKey, txPageData)

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

func GetABIForContract(address common.Address) (*abi.ABI, string, error) {
	cacheKey := fmt.Sprintf("abi:%s", address.String())
	cached, found := cache.Get(cacheKey)
	if found {
		logrus.Infof("retrieved contract abi for address %v from cache", address)

		if cached == nil {
			return nil, "", fmt.Errorf("contract abi not found")
		}
		return cached.(*abi.ABI), "", nil
	}

	//Retrieve metadata.json from sourcify
	abi, name, err := getABIFromSourcify(address)

	if err != nil {
		logrus.Errorf("failed to get abi for contract %v from sourcify: %v", address, err)
		logrus.Error("trying etherscan")

		abi, name, err = getABIFromEtherscan(address)

		if err != nil {
			logrus.Errorf("failed to get abi for contract %v from etherscan: %v", address, err)
			cache.Add(cacheKey, nil)
			return nil, "", fmt.Errorf("contract abi not found")
		}
		cache.Add(cacheKey, abi)
		return abi, name, nil
	}

	cache.Add(cacheKey, abi)
	return abi, name, nil
}

func getABIFromSourcify(address common.Address) (*abi.ABI, string, error) {
	httpClient := http.Client{
		Timeout: time.Second * 5,
	}

	resp, err := httpClient.Get(fmt.Sprintf("https://sourcify.dev/server/repository/contracts/full_match/%d/%s/metadata.json", utils.Config.Chain.DepositChainID, address.String()))
	if err != nil {
		return nil, "", err
	}

	if resp.StatusCode == 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, "", err
		}

		data := &types.SourcifyContractMetadata{}
		err = json.Unmarshal(body, data)
		if err != nil {
			return nil, "", err
		}

		abiString, err := json.Marshal(data.Output.Abi)
		if err != nil {
			return nil, "", err
		}

		contractAbi, err := abi.JSON(bytes.NewReader(abiString))
		if err != nil {
			return nil, "", err
		}

		return &contractAbi, "", nil
	} else {
		return nil, "", fmt.Errorf("sourcify contract code not found")
	}

}

func getABIFromEtherscan(address common.Address) (*abi.ABI, string, error) {
	httpClient := http.Client{
		Timeout: time.Second * 5,
	}

	baseUrl := "api.etherscan.io"

	if utils.Config.Chain.DepositChainID == 5 {
		baseUrl = "api-goerli.etherscan.io"
	}
	resp, err := httpClient.Get(fmt.Sprintf("https://%s/api?module=contract&action=getsourcecode&address=%s&apikey=YourApiKeyToken", baseUrl, address.String()))
	if err != nil {
		return nil, "", err
	}

	if resp.StatusCode == 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, "", err
		}

		data := &types.EtherscanContractMetadata{}
		err = json.Unmarshal(body, data)
		if err != nil {
			return nil, "", err
		}

		contractAbi, err := abi.JSON(strings.NewReader(data.Result[0].Abi))
		if err != nil {
			return nil, "", err
		}

		return &contractAbi, data.Result[0].ContractName, nil
	} else {
		return nil, "", fmt.Errorf("sourcify contract code not found")
	}

}
