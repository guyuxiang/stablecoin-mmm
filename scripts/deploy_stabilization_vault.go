package main

import (
	"context"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	// Config from config.yaml
	rpcURL := "https://astrochain-sepolia.gateway.tenderly.co/5neqYQoinBsj3Cc3O36Dun"
	privateKey := "298149d01f7a23cb938ab6874ea345516479fb70bd5e14c99c0ffaf84798ca80"
	
	// Token addresses
	token0 := "0x2d7efff683b0a21e0989729e0249c42cdf9ee442" // GLUSD
	token1 := "0x948e15b38f096d3a664fdeef44c13709732b2110" // USDT
	fee := uint64(500) // 0.05%
	factory := "0x1F98431c8aD98523631AE4a59f267346ea31F984"
	swapRouter := "0xd1AAE39293221B77B0C71fBD6dCb7Ea29Bb5B166"

	// Connect
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Load private key
	key, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		log.Fatalf("Invalid private key: %v", err)
	}

	chainID, _ := client.NetworkID(context.Background())
	auth, err := bind.NewKeyedTransactorWithChainID(key, chainID)
	if err != nil {
		log.Fatalf("Failed to create transactor: %v", err)
	}
	auth.Nonce = nil
	auth.Value = nil
	auth.GasLimit = 3000000
	auth.GasPrice = nil

	fmt.Println("=== Deployment Config ===")
	fmt.Printf("Chain ID: %s\n", chainID)
	fmt.Printf("Token0: %s\n", token0)
	fmt.Printf("Token1: %s\n", token1)
	fmt.Printf("Fee: %d\n", fee)
	fmt.Printf("Factory: %s\n", factory)
	fmt.Printf("SwapRouter: %s\n", swapRouter)
	fmt.Printf("Deployer: %s\n", auth.From.Hex())

	// For now, just print the config - actual deployment would require compiled contract
	fmt.Println("\n=== Next Steps ===")
	fmt.Println("1. Compile StabilizationVault.sol")
	fmt.Println("2. Deploy using cast or hardhat")
	fmt.Println("3. Call methods to test")

	// Test connection - get balance
	balance, err := client.BalanceAt(context.Background(), auth.From, nil)
	if err != nil {
		log.Printf("Warning: Failed to get balance: %v", err)
	} else {
		fmt.Printf("\nDeployer balance: %s ETH\n", balance.String())
	}

	// Get pool info
	poolAddr := common.HexToAddress("0x12A264f6A787150c6772987e911A1942f1D9411D")
	
	// Simple call to check if pool exists
	var result []byte
	err = client.CallContract(context.Background(), nil, &result)
	if err != nil {
		fmt.Printf("Pool check: %v\n", err)
	}
	
	fmt.Println("\n=== Pool Info ===")
	fmt.Printf("Pool: %s\n", poolAddr.Hex())
}
