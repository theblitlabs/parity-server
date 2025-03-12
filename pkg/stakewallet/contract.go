package stakewallet

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// StakeWalletContract is the Go binding of the StakeWallet contract
type StakeWalletContract struct {
	address common.Address
	backend bind.ContractBackend
	abi     abi.ABI
}

// Your ABI as a string constant
const StakeWalletABI = `[
    {
      "inputs": [
        {
          "internalType": "address",
          "name": "_tokenAddress",
          "type": "address"
        }
      ],
      "stateMutability": "nonpayable",
      "type": "constructor"
    },
    {
      "inputs": [
        {
          "internalType": "address",
          "name": "owner",
          "type": "address"
        }
      ],
      "name": "OwnableInvalidOwner",
      "type": "error"
    },
    {
      "inputs": [
        {
          "internalType": "address",
          "name": "account",
          "type": "address"
        }
      ],
      "name": "OwnableUnauthorizedAccount",
      "type": "error"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "string",
          "name": "deviceId",
          "type": "string"
        },
        {
          "indexed": true,
          "internalType": "address",
          "name": "from",
          "type": "address"
        },
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "amount",
          "type": "uint256"
        }
      ],
      "name": "FundsAdded",
      "type": "event"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "string",
          "name": "deviceId",
          "type": "string"
        },
        {
          "indexed": true,
          "internalType": "address",
          "name": "to",
          "type": "address"
        },
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "amount",
          "type": "uint256"
        }
      ],
      "name": "FundsWithdrawn",
      "type": "event"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "address",
          "name": "previousOwner",
          "type": "address"
        },
        {
          "indexed": true,
          "internalType": "address",
          "name": "newOwner",
          "type": "address"
        }
      ],
      "name": "OwnershipTransferred",
      "type": "event"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "string",
          "name": "creatorDeviceId",
          "type": "string"
        },
        {
          "indexed": true,
          "internalType": "string",
          "name": "solverDeviceId",
          "type": "string"
        },
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "amount",
          "type": "uint256"
        }
      ],
      "name": "TaskPayment",
      "type": "event"
    },
    {
      "anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "address",
          "name": "tokenAddress",
          "type": "address"
        },
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "amount",
          "type": "uint256"
        }
      ],
      "name": "TokenRecovered",
      "type": "event"
    },
    {
      "inputs": [
        {
          "internalType": "uint256",
          "name": "_amount",
          "type": "uint256"
        },
        {
          "internalType": "string",
          "name": "_deviceId",
          "type": "string"
        },
        {
          "internalType": "address",
          "name": "_walletAddress",
          "type": "address"
        }
      ],
      "name": "addFunds",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "_deviceId",
          "type": "string"
        }
      ],
      "name": "getBalance",
      "outputs": [
        {
          "internalType": "uint256",
          "name": "",
          "type": "uint256"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "_deviceId",
          "type": "string"
        }
      ],
      "name": "getWalletInfo",
      "outputs": [
        {
          "internalType": "uint256",
          "name": "balance",
          "type": "uint256"
        },
        {
          "internalType": "string",
          "name": "deviceId",
          "type": "string"
        },
        {
          "internalType": "address",
          "name": "walletAddress",
          "type": "address"
        },
        {
          "internalType": "bool",
          "name": "exists",
          "type": "bool"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
    {
      "inputs": [],
      "name": "owner",
      "outputs": [
        {
          "internalType": "address",
          "name": "",
          "type": "address"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "address",
          "name": "_tokenAddress",
          "type": "address"
        },
        {
          "internalType": "uint256",
          "name": "_amount",
          "type": "uint256"
        }
      ],
      "name": "recoverTokens",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "inputs": [],
      "name": "renounceOwnership",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "inputs": [],
      "name": "token",
      "outputs": [
        {
          "internalType": "contract IERC20",
          "name": "",
          "type": "address"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "address",
          "name": "newOwner",
          "type": "address"
        }
      ],
      "name": "transferOwnership",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "_creatorDeviceId",
          "type": "string"
        },
        {
          "internalType": "string",
          "name": "_solverDeviceId",
          "type": "string"
        },
        {
          "internalType": "uint256",
          "name": "_amount",
          "type": "uint256"
        }
      ],
      "name": "transferPayment",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "_deviceId",
          "type": "string"
        },
        {
          "internalType": "address",
          "name": "_newWalletAddress",
          "type": "address"
        }
      ],
      "name": "updateWalletAddress",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "",
          "type": "string"
        }
      ],
      "name": "wallets",
      "outputs": [
        {
          "internalType": "uint256",
          "name": "balance",
          "type": "uint256"
        },
        {
          "internalType": "string",
          "name": "deviceId",
          "type": "string"
        },
        {
          "internalType": "address",
          "name": "walletAddress",
          "type": "address"
        },
        {
          "internalType": "bool",
          "name": "exists",
          "type": "bool"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "string",
          "name": "_deviceId",
          "type": "string"
        },
        {
          "internalType": "uint256",
          "name": "_amount",
          "type": "uint256"
        }
      ],
      "name": "withdrawFunds",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    }
  ]`

// NewStakeWalletContract creates a new instance of the contract bindings
func NewStakeWalletContract(address common.Address, backend bind.ContractBackend) (*StakeWalletContract, error) {
	contractABI, err := abi.JSON(strings.NewReader(StakeWalletABI))
	if err != nil {
		return nil, err
	}

	return &StakeWalletContract{
		address: address,
		backend: backend,
		abi:     contractABI,
	}, nil
}

// GetStakeInfo retrieves stake information for a given device ID
func (c *StakeWalletContract) GetStakeInfo(opts *bind.CallOpts, deviceID string) (StakeInfo, error) {
	var out []interface{}
	err := bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend).
		Call(opts, &out, "getWalletInfo", deviceID)
	if err != nil {
		return StakeInfo{}, err
	}

	return StakeInfo{
		Amount:        abi.ConvertType(out[0], new(big.Int)).(*big.Int),
		DeviceID:      *abi.ConvertType(out[1], new(string)).(*string),
		WalletAddress: *abi.ConvertType(out[2], new(common.Address)).(*common.Address),
		Exists:        *abi.ConvertType(out[3], new(bool)).(*bool),
	}, nil
}

// Stake tokens with device ID
func (c *StakeWalletContract) Stake(opts *bind.TransactOpts, amount *big.Int, deviceID string, walletAddr common.Address) (*types.Transaction, error) {
	return bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend).
		Transact(opts, "addFunds", amount, deviceID, walletAddr)
}

// DistributeStake distributes stake between user and recipient
func (c *StakeWalletContract) TransferPayment(
	opts *bind.TransactOpts,
	creatorDeviceID string,
	solverDeviceID string,
	amount *big.Int,
) (*types.Transaction, error) {
	return bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend).
		Transact(opts, "transferPayment", creatorDeviceID, solverDeviceID, amount)
}

// Owner returns the contract owner
func (c *StakeWalletContract) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend).
		Call(opts, &out, "owner")
	if err != nil {
		return common.Address{}, err
	}
	return *abi.ConvertType(out[0], new(common.Address)).(*common.Address), nil
}

// Token returns the staking token address
func (c *StakeWalletContract) Token(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend).
		Call(opts, &out, "token")
	if err != nil {
		return common.Address{}, err
	}
	return *abi.ConvertType(out[0], new(common.Address)).(*common.Address), nil
}

// GetBalanceByDeviceID retrieves balance for a device ID
func (c *StakeWalletContract) GetBalanceByDeviceID(opts *bind.CallOpts, deviceID string) (*big.Int, error) {
	var out []interface{}
	err := bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend).
		Call(opts, &out, "getBalance", deviceID)
	if err != nil {
		return nil, err
	}
	return abi.ConvertType(out[0], new(big.Int)).(*big.Int), nil
}

// GetUsersByDeviceId retrieves users for a device ID
func (c *StakeWalletContract) GetUsersByDeviceId(opts *bind.CallOpts, deviceID string) ([]common.Address, error) {
	var out []interface{}
	err := bind.NewBoundContract(c.address, c.abi, c.backend, c.backend, c.backend).
		Call(opts, &out, "getUsersByDeviceId", deviceID)
	if err != nil {
		return nil, err
	}
	return *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address), nil
}

// CheckStakeExists checks if a user has staked tokens
func (c *StakeWalletContract) CheckStakeExists(opts *bind.CallOpts, user common.Address) (bool, error) {
	stakeInfo, err := c.GetStakeInfo(opts, user.String())
	if err != nil {
		return false, err
	}
	return stakeInfo.Exists, nil
}
