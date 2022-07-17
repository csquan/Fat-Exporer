package eth1data

import (
	"bytes"
	"context"
	"encoding/json"
	"eth2-exporter/db"
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
	"github.com/go-redis/cache/v8"
	"github.com/sirupsen/logrus"
)

var client *ethclient.Client

func Init() error {
	c, err := ethclient.Dial(utils.Config.Frontend.Eth1Data.Eth1ArchiveNodeEndpoint)

	if err != nil {
		return err
	}

	client = c
	return nil
}

func GetEth1Transaction(hash common.Hash) (*types.Eth1TxData, error) {
	cacheKey := fmt.Sprintf("tx:%s", hash.String())
	wanted := &types.Eth1TxData{}
	if err := db.RedisCache.Get(context.Background(), cacheKey, wanted); err == nil {
		logrus.Infof("retrieved data for tx %v from cache", hash)

		return wanted, nil
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

	txPageData.To = tx.To()

	if txPageData.To == nil {
		txPageData.To = &receipt.ContractAddress
		txPageData.IsContractCreation = true
	}
	code, err := GetCodeAt(*txPageData.To)
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
			meta, err := GetMetadataForContract(log.Address)

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
				txPageData.ToName = meta.Name
				boundContract := bind.NewBoundContract(*txPageData.To, *meta.ABI, nil, nil, nil)

				for name, event := range meta.ABI.Events {
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

	err = db.RedisCache.Set(&cache.Item{
		Ctx:   context.Background(),
		Key:   cacheKey,
		Value: txPageData,
		TTL:   0,
	})
	if err != nil {
		return nil, fmt.Errorf("error writing data for tx %v to cache: %v", hash, err)
	}

	return txPageData, nil
}

func GetCodeAt(address common.Address) ([]byte, error) {
	cacheKey := fmt.Sprintf("a:%s", address.String())
	wanted := []byte{}
	if err := db.RedisCache.Get(context.Background(), cacheKey, &wanted); err == nil {
		logrus.Infof("retrieved code data for address %v from cache", address)

		return wanted, nil
	}

	code, err := client.CodeAt(context.Background(), address, nil)
	if err != nil {
		return nil, fmt.Errorf("error retrieving code data for address %v: %v", address, err)
	}

	err = db.RedisCache.Set(&cache.Item{
		Ctx:   context.Background(),
		Key:   cacheKey,
		Value: code,
		TTL:   0,
	})
	if err != nil {
		return nil, fmt.Errorf("error writing code data for address %v to cache: %v", address, err)
	}

	return code, nil
}

func GetBlockHeaderByHash(hash common.Hash) (*geth_types.Header, error) {
	cacheKey := fmt.Sprintf("h:%s", hash.String())

	wanted := &geth_types.Header{}
	if err := db.RedisCache.Get(context.Background(), cacheKey, &wanted); err == nil {
		logrus.Infof("retrieved header data for block %v from cache", hash)
		return wanted, nil
	}

	header, err := client.HeaderByHash(context.Background(), hash)
	if err != nil {
		return nil, fmt.Errorf("error retrieving block header data for tx %v: %v", hash, err)
	}

	err = db.RedisCache.Set(&cache.Item{
		Ctx:   context.Background(),
		Key:   cacheKey,
		Value: header,
		TTL:   0,
	})
	if err != nil {
		return nil, fmt.Errorf("error writing header data for block %v to cache: %v", hash, err)
	}

	return header, nil
}

func GetTransactionReceipt(hash common.Hash) (*geth_types.Receipt, error) {
	cacheKey := fmt.Sprintf("r:%s", hash.String())

	wanted := &geth_types.Receipt{}
	if err := db.RedisCache.Get(context.Background(), cacheKey, &wanted); err == nil {
		logrus.Infof("retrieved receipt data for tx %v from cache", hash)
		return wanted, nil
	}

	receipt, err := client.TransactionReceipt(context.Background(), hash)
	if err != nil {
		return nil, fmt.Errorf("error retrieving receipt data for tx %v: %v", hash, err)
	}

	err = db.RedisCache.Set(&cache.Item{
		Ctx:   context.Background(),
		Key:   cacheKey,
		Value: receipt,
		TTL:   0,
	})
	if err != nil {
		return nil, fmt.Errorf("error writing receipt data for tx %v to cache: %v", hash, err)
	}

	return receipt, nil
}

func GetMetadataForContract(address common.Address) (*types.AddressMetadata, error) {
	cacheKey := fmt.Sprintf("meta:%s", address.String())

	wanted := &types.AddressMetadata{}
	if err := db.RedisCache.Get(context.Background(), cacheKey, wanted); err == nil {

		if wanted.ABI == nil {
			return nil, fmt.Errorf("contract abi not found")
		}
		logrus.Infof("retrieved metadata for address %v from cache", address)
		return wanted, nil
	} else if err != cache.ErrCacheMiss {
		logrus.Fatal(err)
	}

	//Retrieve metadata.json from sourcify
	abi, name, err := getABIFromSourcify(address)

	if err != nil {
		logrus.Errorf("failed to get abi for contract %v from sourcify: %v", address, err)
		logrus.Error("trying etherscan")

		abi, name, err = getABIFromEtherscan(address)

		if err != nil {
			logrus.Errorf("failed to get abi for contract %v from etherscan: %v", address, err)
			err = db.RedisCache.Set(&cache.Item{
				Ctx:   context.Background(),
				Key:   cacheKey,
				Value: "",
				TTL:   time.Hour * 24,
			})
			if err != nil {
				return nil, fmt.Errorf("error writing addresss metadata for address %v to cache: %v", address, err)
			}
			return nil, fmt.Errorf("contract abi not found")
		}
		meta := &types.AddressMetadata{
			ABI:  abi,
			Name: name,
		}
		err = db.RedisCache.Set(&cache.Item{
			Ctx:   context.Background(),
			Key:   cacheKey,
			Value: meta,
			TTL:   time.Hour * 24,
		})
		if err != nil {
			return nil, fmt.Errorf("error writing addresss metadata for address %v to cache: %v", address, err)
		}
		return meta, nil
	}

	meta := &types.AddressMetadata{
		ABI:  abi,
		Name: name,
	}
	err = db.RedisCache.Set(&cache.Item{
		Ctx:   context.Background(),
		Key:   cacheKey,
		Value: meta,
		TTL:   time.Hour * 24,
	})
	if err != nil {
		return nil, fmt.Errorf("error writing addresss metadata for address %v to cache: %v", address, err)
	}
	return meta, nil
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
	resp, err := httpClient.Get(fmt.Sprintf("https://%s/api?module=contract&action=getsourcecode&address=%s&apikey=%s", baseUrl, address.String(), utils.Config.Frontend.Eth1Data.EtherscanAPIKey))
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
		return nil, "", fmt.Errorf("etherscan contract code not found")
	}
}
