package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	geth_types "github.com/ethereum/go-ethereum/core/types"
)

// TxPageData is a struct to hold detailed tx data for the tx page
type Eth1TxData struct {
	From             common.Address
	FromName         string
	ToName           string
	GethTx           *geth_types.Transaction
	Receipt          *geth_types.Receipt
	Header           *geth_types.Header
	IsPending        bool
	TargetIsContract bool
	TxFee            *big.Int
	Events           []*Eth1EventData
}

type Eth1EventData struct {
	Address     common.Address
	Name        string
	Topics      []common.Hash
	Data        []byte
	DecodedData map[string]string
}

type SourcifyContractMetadata struct {
	Compiler struct {
		Version string `json:"version"`
	} `json:"compiler"`
	Language string `json:"language"`
	Output   struct {
		Abi []struct {
			Anonymous bool `json:"anonymous"`
			Inputs    []struct {
				Indexed      bool   `json:"indexed"`
				InternalType string `json:"internalType"`
				Name         string `json:"name"`
				Type         string `json:"type"`
			} `json:"inputs"`
			Name    string `json:"name"`
			Outputs []struct {
				InternalType string `json:"internalType"`
				Name         string `json:"name"`
				Type         string `json:"type"`
			} `json:"outputs"`
			StateMutability string `json:"stateMutability"`
			Type            string `json:"type"`
		} `json:"abi"`
		Devdoc struct {
			Methods struct {
				Allowance_address_address struct {
					Details string `json:"details"`
				} `json:"allowance(address,address)"`
				Approve_address_uint256 struct {
					Details string `json:"details"`
				} `json:"approve(address,uint256)"`
				BalanceOf_address struct {
					Details string `json:"details"`
				} `json:"balanceOf(address)"`
				Constructor struct {
					Details string `json:"details"`
				} `json:"constructor"`
				Name struct {
					Details string `json:"details"`
				} `json:"name()"`
				Symbol struct {
					Details string `json:"details"`
				} `json:"symbol()"`
				TotalSupply struct {
					Details string `json:"details"`
				} `json:"totalSupply()"`
				Transfer_address_uint256 struct {
					Details string `json:"details"`
				} `json:"transfer(address,uint256)"`
			} `json:"methods"`
		} `json:"devdoc"`
		Userdoc struct {
			Methods struct{} `json:"methods"`
		} `json:"userdoc"`
	} `json:"output"`
	Settings struct {
		CompilationTarget struct {
			Browser_Stakehavens_sol string `json:"browser/Stakehavens.sol"`
		} `json:"compilationTarget"`
		EvmVersion string   `json:"evmVersion"`
		Libraries  struct{} `json:"libraries"`
		Metadata   struct {
			BytecodeHash string `json:"bytecodeHash"`
		} `json:"metadata"`
		Optimizer struct {
			Enabled bool  `json:"enabled"`
			Runs    int64 `json:"runs"`
		} `json:"optimizer"`
		Remappings []interface{} `json:"remappings"`
	} `json:"settings"`
	Sources struct {
		Browser_Stakehavens_sol struct {
			Keccak256 string   `json:"keccak256"`
			Urls      []string `json:"urls"`
		} `json:"browser/Stakehavens.sol"`
	} `json:"sources"`
	Version int64 `json:"version"`
}

type EtherscanContractMetadata struct {
	Message string `json:"message"`
	Result  []struct {
		Abi                  string `json:"ABI"`
		CompilerVersion      string `json:"CompilerVersion"`
		ConstructorArguments string `json:"ConstructorArguments"`
		ContractName         string `json:"ContractName"`
		EVMVersion           string `json:"EVMVersion"`
		Implementation       string `json:"Implementation"`
		Library              string `json:"Library"`
		LicenseType          string `json:"LicenseType"`
		OptimizationUsed     string `json:"OptimizationUsed"`
		Proxy                string `json:"Proxy"`
		Runs                 string `json:"Runs"`
		SourceCode           string `json:"SourceCode"`
		SwarmSource          string `json:"SwarmSource"`
	} `json:"result"`
	Status string `json:"status"`
}
