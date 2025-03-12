package wallet

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// ERC20 represents the ParityToken interface
type ERC20 interface {
	// Read-only methods
	Name(opts *bind.CallOpts) (string, error)
	Symbol(opts *bind.CallOpts) (string, error)
	Decimals(opts *bind.CallOpts) (uint8, error)
	TotalSupply(opts *bind.CallOpts) (*big.Int, error)
	BalanceOf(opts *bind.CallOpts, account common.Address) (*big.Int, error)
	Allowance(opts *bind.CallOpts, owner common.Address, spender common.Address) (*big.Int, error)

	// Transaction methods
	Transfer(opts *bind.TransactOpts, to common.Address, value *big.Int) (*types.Transaction, error)
	Approve(opts *bind.TransactOpts, spender common.Address, value *big.Int) (*types.Transaction, error)
	TransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, value *big.Int) (*types.Transaction, error)
	Mint(opts *bind.TransactOpts, to common.Address, value *big.Int) (*types.Transaction, error)
	Burn(opts *bind.TransactOpts, value *big.Int) (*types.Transaction, error)
	TransferWithData(opts *bind.TransactOpts, to common.Address, value *big.Int, data []byte) (*types.Transaction, error)
	TransferWithDataAndCallback(opts *bind.TransactOpts, to common.Address, value *big.Int, data []byte) (*types.Transaction, error)
}

// NewERC20 creates a new instance of ERC20
func NewERC20(address common.Address, backend bind.ContractBackend) (ERC20, error) {
	return NewParityToken(address, backend)
}
