package contracts

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// StabilizationVaultABI is the ABI for StabilizationVault contract
var StabilizationVaultABI, _ = abi.JSON([]byte(`[
	{
		"inputs": [
			{"name": "_token0", "type": "address"},
			{"name": "_token1", "type": "address"},
			{"name": "_pool", "type": "address"},
			{"name": "_swapRouter", "type": "address"},
			{"name": "initialOwner", "type": "address"}
		],
		"stateMutability": "nonpayable",
		"type": "constructor"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "user", "type": "address"},
			{"name": "amount0", "type": "uint256"},
			{"name": "amount1", "type": "uint256"},
			{"name": "shares", "type": "uint256"}
		],
		"name": "Deposit",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "user", "type": "address"},
			{"name": "amount0", "type": "uint256"},
			{"name": "amount1", "type": "uint256"},
			{"name": "shares", "type": "uint256"}
		],
		"name": "Withdraw",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"name": "amountIn", "type": "uint256"},
			{"name": "amountOut", "type": "uint256"},
			{"name": "profit", "type": "uint256"},
			{"name": "success", "type": "bool"}
		],
		"name": "ArbitrageExecuted",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"name": "newReserve0", "type": "uint256"},
			{"name": "newReserve1", "type": "uint256"}
		],
		"name": "ReserveUpdated",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"name": "reason", "type": "string"}
		],
		"name": "CircuitBreakerTriggered",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"name": "param", "type": "string"},
			{"name": "value", "type": "uint256"}
		],
		"name": "ParametersUpdated",
		"type": "event"
	},
	{
		"inputs": [],
		"name": "token0",
		"outputs": [{"name": "", "type": "address"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "token1",
		"outputs": [{"name": "", "type": "address"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "pool",
		"outputs": [{"name": "", "type": "address"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "swapRouter",
		"outputs": [{"name": "", "type": "address"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "reserve0",
		"outputs": [{"name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "reserve1",
		"outputs": [{"name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "totalShares",
		"outputs": [{"name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [{"name": "", "type": "address"}],
		"name": "shares",
		"outputs": [{"name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "minProfitThreshold",
		"outputs": [{"name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "maxSlippageBps",
		"outputs": [{"name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "maxSwapAmount",
		"outputs": [{"name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "circuitBreakerActive",
		"outputs": [{"name": "", "type": "bool"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "cooldownPeriod",
		"outputs": [{"name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "amount0", "type": "uint256"},
			{"name": "amount1", "type": "uint256"}
		],
		"name": "deposit",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"name": "shareAmount", "type": "uint256"}],
		"name": "withdraw",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "getPrice",
		"outputs": [
			{"name": "sqrtPriceX96", "type": "uint160"},
			{"name": "tick", "type": "int24"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "getLiquidity",
		"outputs": [{"name": "liquidity", "type": "uint128"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [{"name": "targetPrice", "type": "uint256"}],
		"name": "calculateSwapAmount",
		"outputs": [{"name": "amountIn", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "amountIn", "type": "uint256"},
			{"name": "minAmountOut", "type": "uint256"},
			{"name": "tokenIn", "type": "address"},
			{"name": "tokenOut", "type": "address"}
		],
		"name": "executeArbitrage",
		"outputs": [
			{"name": "success", "type": "bool"},
			{"name": "profit", "type": "uint256"}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"name": "reason", "type": "string"}],
		"name": "triggerCircuitBreaker",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "releaseCircuitBreaker",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"name": "_threshold", "type": "uint256"}],
		"name": "setMinProfitThreshold",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"name": "_slippage", "type": "uint256"}],
		"name": "setMaxSlippageBps",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"name": "_maxAmount", "type": "uint256"}],
		"name": "setMaxSwapAmount",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"name": "_period", "type": "uint256"}],
		"name": "setCooldownPeriod",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "getReserves",
		"outputs": [
			{"name": "", "type": "uint256"},
			{"name": "", "type": "uint256"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [{"name": "user", "type": "address"}],
		"name": "getShareBalance",
		"outputs": [{"name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "getVaultTVL",
		"outputs": [{"name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	}
]`))

// StabilizationVault is a Go wrapper for the StabilizationVault contract
type StabilizationVault struct {
	addr  common.Address
	client *ethclient.Client
}

// NewStabilizationVault creates a new instance
func NewStabilizationVault(addr common.Address, client *ethclient.Client) *StabilizationVault {
	return &StabilizationVault{
		addr:  addr,
		client: client,
	}
}

// VaultState represents the current state of the vault
type VaultState struct {
	Reserve0           *big.Int
	Reserve1           *big.Int
	TotalShares        *big.Int
	MinProfitThreshold *big.Int
	RebalanceThresholdBps *big.Int
	TargetPrice           *big.Int
	MaxSlippageBps     *big.Int
	MaxSwapAmount      *big.Int
	CircuitBreaker     bool
	CooldownPeriod     *big.Int
}

// PoolState represents Uniswap pool state
type PoolState struct {
	SqrtPriceX96 *big.Int
	Tick         *big.Int
	Liquidity    *big.Int
}

// GetVaultState returns current vault state
func (v *StabilizationVault) GetVaultState(ctx context.Context) (*VaultState, error) {
	methods := []struct {
		name string
		out  interface{}
	}{
		{"reserve0", new(*big.Int)},
		{"reserve1", new(*big.Int)},
		{"totalShares", new(*big.Int)},
		{"minProfitThreshold", new(*big.Int)},
		{"maxSlippageBps", new(*big.Int)},
		{"maxSwapAmount", new(*big.Int)},
		{"circuitBreakerActive", new(bool)},
		{"cooldownPeriod", new(*big.Int)},
	}

	results := make([]interface{}, len(methods))
	for i, m := range methods {
		result, err := v.client.CallContract(ctx, CallMsg{
			To:   &v.addr,
			Data: MustPackMethod(StabilizationVaultABI, m.name),
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("call %s failed: %w", m.name, err)
		}
		if err := StabilizationVaultABI.UnpackIntoInterface(m.out, m.name, result); err != nil {
			return nil, err
		}
		results[i] = m.out
	}

	return &VaultState{
		Reserve0:           *results[0].(*big.Int),
		Reserve1:           *results[1].(*big.Int),
		TotalShares:        *results[2].(*big.Int),
		MinProfitThreshold: *results[3].(*big.Int),
		MaxSlippageBps:     *results[4].(*big.Int),
		MaxSwapAmount:      *results[5].(*big.Int),
		CircuitBreaker:     *results[6].(*bool),
		CooldownPeriod:     *results[7].(*big.Int),
	}, nil
}

// GetPoolState returns current Uniswap pool state
func (v *StabilizationVault) GetPoolState(ctx context.Context, poolAddr common.Address) (*PoolState, error) {
	// Call slot0
	slot0Result, err := v.client.CallContract(ctx, CallMsg{
		To: &poolAddr,
		Data: common.FromHex("0x3850c7bd"), // slot0()
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("slot0 call failed: %w", err)
	}

	var sqrtPriceX96, tick *big.Int
	if len(slot0Result) >= 64 {
		sqrtPriceX96 = new(big.Int).SetBytes(slot0Result[:32])
		tick = new(big.Int).SetBytes(slot0Result[32:64])
	}

	// Call liquidity
	liquidityResult, err := v.client.CallContract(ctx, CallMsg{
		To: &poolAddr,
		Data: common.FromHex("0xec9c4d3"), // liquidity()
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("liquidity call failed: %w", err)
	}

	var liquidity *big.Int
	if len(liquidityResult) >= 32 {
		liquidity = new(big.Int).SetBytes(liquidityResult)
	}

	return &PoolState{
		SqrtPriceX96: sqrtPriceX96,
		Tick:         tick,
		Liquidity:    liquidity,
	}, nil
}

// CalculateSwapAmount calculates the swap amount needed to reach target price
func (v *StabilizationVault) CalculateSwapAmount(ctx context.Context, poolAddr common.Address, targetPrice *big.Int) (*big.Int, error) {
	result, err := v.client.CallContract(ctx, CallMsg{
		To: &v.addr,
		Data: MustPackMethod(StabilizationVaultABI, "calculateSwapAmount", targetPrice),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("calculateSwapAmount failed: %w", err)
	}

	var amountIn *big.Int
	if err := StabilizationVaultABI.UnpackIntoInterface(&amountIn, "calculateSwapAmount", result); err != nil {
		return nil, err
	}

	return amountIn, nil
}

// GetPriceDeviation returns current price deviation
func (v *StabilizationVault) GetPriceDeviation(ctx context.Context) (*big.Int, bool, error) {
	result, err := v.client.CallContract(ctx, CallMsg{
		To: &v.addr,
		Data: MustPackMethod(StabilizationVaultABI, "getPriceDeviation"),
	}, nil)
	if err != nil {
		return nil, false, fmt.Errorf("getPriceDeviation failed: %w", err)
	}

	var deviation, threshold *big.Int
	var aboveThreshold bool
	if len(result) >= 64 {
		deviation = new(big.Int).SetBytes(result[:32])
		threshold = new(big.Int).SetBytes(result[32:64])
		aboveThreshold = result[63] == 1
	}

	return deviation, aboveThreshold, nil
}

// SetRebalanceThresholdBps sets the price deviation threshold
func (v *StabilizationVault) SetRebalanceThresholdBps(auth *bind.TransactOpts, threshold *big.Int) (*types.Transaction, error) {
	tx, err := bind.NewBoundContract(v.addr, StabilizationVaultABI, v.client, v.client, auth).Transact(
		"setRebalanceThresholdBps",
		auth,
		threshold,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set threshold: %w", err)
	}
	return tx, nil
}

// SetTargetPrice sets the target price
func (v *StabilizationVault) SetTargetPrice(auth *bind.TransactOpts, targetPrice *big.Int) (*types.Transaction, error) {
	tx, err := bind.NewBoundContract(v.addr, StabilizationVaultABI, v.client, v.client, auth).Transact(
		"setTargetPrice",
		auth,
		targetPrice,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set target price: %w", err)
	}
	return tx, nil
}

// CallMsg is a minimal version of ethereum.CallMsg
type CallMsg struct {
	From     common.Address
	To       *common.Address
	Gas      uint64
	GasPrice *big.Int
	Value    *big.Int
	Data     []byte
}

// MustPackMethod packs a method call
func MustPackMethod(abi abi.ABI, name string, args ...interface{}) []byte {
	method, err := abi.Pack(name, args...)
	if err != nil {
		panic(err)
	}
	return method
}
