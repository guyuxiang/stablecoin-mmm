package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	uniswapbot "stablecoin-mmm/pkg/uniswap"
)

func main() {
	// Configuration
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		rpcURL = "https://astrochain-sepolia.gateway.tenderly.co/5neqYQoinBsj3Cc3O36Dun"
	}

	privateKey := os.Getenv("PRIVATE_KEY")
	if privateKey == "" {
		log.Fatal("PRIVATE_KEY not set")
	}

	// Connect to network
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("Failed to connect to network: %v", err)
	}

	// Load private key
	key, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		log.Fatalf("Invalid private key: %v", err)
	}

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		log.Fatalf("Failed to get chain ID: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(key, chainID)
	if err != nil {
		log.Fatalf("Failed to create transactor: %v", err)
	}

	// Token addresses (Unichain Sepolia)
	token0Address := common.HexToAddress("0x") // USDT
	token1Address := common.HexToAddress("0x") // USDx
	poolAddress := common.HexToAddress("0x12A264f6A787150c6772987e911A1942f1D9411D") // GLUSD/USDT
	routerAddress := common.HexToAddress("")   // Uniswap V3 SwapRouter

	// For now, print deployment info
	fmt.Println("=== Stabilization Vault Deployment ===Chain ID:", chain")
	fmt.Println("ID)
	fmt.Println("Token0 (USDT):", token0Address.Hex())
	fmt.Println("Token1 (USDx):", token1Address.Hex())
	fmt.Println("Pool:", poolAddress.Hex())
	fmt.Println("Router:", routerAddress.Hex())
	fmt.Println("Deployer:", auth.From.Hex())

	// Test Uniswap client
	uniswapClient, err := uniswap.NewClient(rpcURL, poolAddress.Hex())
	if err != nil {
		log.Printf("Warning: Failed to create Uniswap client: %v", err)
	} else {
		ctx := context.Background()

		// Get slot0
		slot0, err := uniswapClient.GetSlot0(ctx)
		if err != nil {
			log.Printf("Warning: Failed to get slot0: %v", err)
		} else {
			fmt.Println("\n=== Pool State ===")
			fmt.Printf("SqrtPriceX96: %s\n", slot0.SqrtPriceX96.String())
			fmt.Printf("Tick: %d\n", slot0.Tick)
			fmt.Printf("Liquidity: %s\n", slot0.Liquidity.String())

			// Calculate price
			price := new(big.Float).Quo(
				new(big.Float).SetInt(slot0.SqrtPriceX96),
				new(big.Float).SetInt(big.NewInt(1<<96)),
			)
			price = new(big.Float).Mul(price, price)
			priceStr, _ := price.Float64()
			fmt.Printf("Current Price (token1/token0): %.6f\n", priceStr)
		}
	}

	fmt.Println("\n=== Next Steps ===")
	fmt.Println("1. Deploy StabilizationVault contract")
	fmt.Println("2. Fund the vault with USDT and USDx")
	fmt.Println("3. Configure bot to call executeArbitrage()")
}

// Helper function for sqrt
func sqrt(n *big.Int) *big.Int {
	if n.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0)
	}

	a := new(big.Int).Set(n)
	b := new(big.Int).Add(new(big.Int).Div(n, big.NewInt(2)), big.NewInt(1))

	for b.Cmp(a) < 0 {
		a, b = b, new(big.Int).Add(b, new(big.Int).Div(n, b))
		b = new(big.Int).Div(new(big.Int).Add(b, a), big.NewInt(2))
	}

	return a
}
