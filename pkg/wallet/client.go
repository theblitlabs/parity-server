package wallet

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/theblitlabs/keystore"
)

type Client struct {
	*ethclient.Client
	chainID    *big.Int
	auth       *bind.TransactOpts
	privateKey *ecdsa.PrivateKey
	address    common.Address
}

func NewClient(rpcURL string, chainID int64) (*Client, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}

	// Load private key from keystore
	privateKey, err := keystore.LoadPrivateKey()

	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}

	// Create transaction auth
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(chainID))
	if err != nil {
		return nil, fmt.Errorf("failed to create transactor: %w", err)
	}

	return &Client{
		Client:     client,
		chainID:    big.NewInt(chainID),
		auth:       auth,
		privateKey: privateKey,
		address:    crypto.PubkeyToAddress(privateKey.PublicKey),
	}, nil
}

func (c *Client) GetERC20Balance(contractAddr, walletAddr common.Address) (*big.Int, error) {
	token, err := NewERC20(contractAddr, c.Client)
	if err != nil {
		return nil, err
	}

	return token.BalanceOf(&bind.CallOpts{}, walletAddr)
}

// Additional helper methods for ParityToken functionality
func (c *Client) GetTokenInfo(contractAddr common.Address) (name string, symbol string, decimals uint8, err error) {
	token, err := NewERC20(contractAddr, c.Client)
	if err != nil {
		return "", "", 0, err
	}

	name, err = token.Name(&bind.CallOpts{})
	if err != nil {
		return "", "", 0, err
	}

	symbol, err = token.Symbol(&bind.CallOpts{})
	if err != nil {
		return "", "", 0, err
	}

	decimals, err = token.Decimals(&bind.CallOpts{})
	if err != nil {
		return "", "", 0, err
	}

	return name, symbol, decimals, nil
}

func (c *Client) GetAllowance(contractAddr, owner, spender common.Address) (*big.Int, error) {
	token, err := NewERC20(contractAddr, c.Client)
	if err != nil {
		return nil, err
	}

	return token.Allowance(&bind.CallOpts{}, owner, spender)
}

func (c *Client) GetTotalSupply(contractAddr common.Address) (*big.Int, error) {
	token, err := NewERC20(contractAddr, c.Client)
	if err != nil {
		return nil, err
	}

	return token.TotalSupply(&bind.CallOpts{})
}

// TransferToken transfers tokens from the authorized account to the recipient
func (c *Client) TransferToken(auth *bind.TransactOpts, contractAddr, to common.Address, amount *big.Int) (*types.Transaction, error) {
	token, err := NewERC20(contractAddr, c.Client)
	if err != nil {
		return nil, err
	}
	return token.Transfer(auth, to, amount)
}

// ApproveToken approves the spender to spend tokens on behalf of the authorized account
func (c *Client) ApproveToken(auth *bind.TransactOpts, contractAddr, spender common.Address, amount *big.Int) (*types.Transaction, error) {
	token, err := NewERC20(contractAddr, c.Client)
	if err != nil {
		return nil, err
	}
	return token.Approve(auth, spender, amount)
}

// TransferFromToken transfers tokens from one address to another using allowance
func (c *Client) TransferFromToken(auth *bind.TransactOpts, contractAddr, from, to common.Address, amount *big.Int) (*types.Transaction, error) {
	token, err := NewERC20(contractAddr, c.Client)
	if err != nil {
		return nil, err
	}
	return token.TransferFrom(auth, from, to, amount)
}

// MintToken mints new tokens to the specified address
func (c *Client) MintToken(auth *bind.TransactOpts, contractAddr, to common.Address, amount *big.Int) (*types.Transaction, error) {
	token, err := NewERC20(contractAddr, c.Client)
	if err != nil {
		return nil, err
	}
	return token.Mint(auth, to, amount)
}

// BurnToken burns tokens from the authorized account
func (c *Client) BurnToken(auth *bind.TransactOpts, contractAddr common.Address, amount *big.Int) (*types.Transaction, error) {
	token, err := NewERC20(contractAddr, c.Client)
	if err != nil {
		return nil, err
	}
	return token.Burn(auth, amount)
}

// TransferTokenWithData transfers tokens with additional data
func (c *Client) TransferTokenWithData(auth *bind.TransactOpts, contractAddr, to common.Address, amount *big.Int, data []byte) (*types.Transaction, error) {
	token, err := NewERC20(contractAddr, c.Client)
	if err != nil {
		return nil, err
	}
	return token.TransferWithData(auth, to, amount, data)
}

// TransferTokenWithDataAndCallback transfers tokens with data and executes a callback on the recipient
func (c *Client) TransferTokenWithDataAndCallback(auth *bind.TransactOpts, contractAddr, to common.Address, amount *big.Int, data []byte) (*types.Transaction, error) {
	token, err := NewERC20(contractAddr, c.Client)
	if err != nil {
		return nil, err
	}
	return token.TransferWithDataAndCallback(auth, to, amount, data)
}

func (c *Client) Address() common.Address {
	return c.address
}

func (c *Client) GetTransactOpts() (*bind.TransactOpts, error) {
	if c.auth == nil {
		return nil, fmt.Errorf("wallet not authenticated")
	}
	return c.auth, nil
}

// NewClientWithKey creates a client with a specific private key
func NewClientWithKey(rpcURL string, chainID *big.Int, privateKey string) (*Client, error) {
	key, err := crypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	client, err := NewClient(rpcURL, chainID.Int64())
	if err != nil {
		return nil, err
	}

	client.privateKey = key
	client.address = crypto.PubkeyToAddress(key.PublicKey)

	return client, nil
}
