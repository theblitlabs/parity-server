package stakewallet

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// StakeInfo represents the stake information structure from the contract
type StakeInfo struct {
	Amount        *big.Int
	DeviceID      string
	WalletAddress common.Address
	Exists        bool
}

// StakeWallet represents the staking contract interface
type StakeWallet interface {
	// Read-only methods
	// Note: deviceID parameters are plain strings, not hashed
	GetBalanceByDeviceID(opts *bind.CallOpts, deviceID string) (*big.Int, error)
	GetStakeInfo(opts *bind.CallOpts, deviceID string) (StakeInfo, error)
	Owner(opts *bind.CallOpts) (common.Address, error)
	Token(opts *bind.CallOpts) (common.Address, error)

	// Transaction methods
	Stake(opts *bind.TransactOpts, amount *big.Int, deviceID string, walletAddr common.Address) (*types.Transaction, error)
	TransferPayment(
		opts *bind.TransactOpts,
		creatorDeviceID string,
		solverDeviceID string,
		amount *big.Int,
	) (*types.Transaction, error)
}

// NewStakeWallet creates a new instance of StakeWallet
func NewStakeWallet(address common.Address, backend bind.ContractBackend) (StakeWallet, error) {
	return NewStakeWalletContract(address, backend)
}
