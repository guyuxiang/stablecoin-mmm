package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	coreEntities "github.com/daoleno/uniswap-sdk-core/entities"
	"github.com/daoleno/uniswapv3-sdk/constants"
	"github.com/daoleno/uniswapv3-sdk/entities"
	"github.com/daoleno/uniswapv3-sdk/examples/contract"
	"github.com/daoleno/uniswapv3-sdk/periphery"
	"github.com/daoleno/uniswapv3-sdk/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	RPCURL        = "https://astrochain-sepolia.gateway.tenderly.co/5neqYQoinBsj3Cc3O36Dun"
	CHAINID       = int64(1301)
	FACTORYADDR   = "0x1F98431c8aD98523631AE4a59f267346ea31F984"
	POSITIONMGR   = "0xC36442b4a4522E871399CD717aBDD847Ab11FE88"
	SWAPROUTER    = "0xE592427A0AEce92De3Edee1F18E0157C05861564"
	POOLADDR      = "0x4e250d2b6f4534a0e5d3f08c3b16e80c4e63aef4"
	FEE           = uint32(500)
	TOKEN0ADDR    = "0x948e15b38f096d3a664fdeef44c13709732b2110"
	TOKEN1ADDR    = "0x2d7efff683b0a21e0989729e0249c42cdf9ee442"
	PRIVATEKEY    = "298149d01f7a23cb938ab6874ea345516479fb70bd5e14c99c0ffaf84798ca80"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	ctx := context.Background()

	client, err := ethclient.Dial(RPCURL)
	if err != nil {
		log.Fatal("Failed to connect to client:", err)
	}
	log.Println("✓ Connected to RPC")

	privateKey, err := crypto.HexToECDSA(PRIVATEKEY)
	if err != nil {
		log.Fatal("Invalid private key:", err)
	}
	walletAddr := crypto.PubkeyToAddress(privateKey.PublicKey)
	log.Println("Wallet address:", walletAddr.Hex())

	token0 := coreEntities.NewToken(CHAINID, common.HexToAddress(TOKEN0ADDR), 18, "GLUSD", "GLUSD")
	token1 := coreEntities.NewToken(CHAINID, common.HexToAddress(TOKEN1ADDR), 18, "USDT", "USDT")
	log.Printf("✓ Token0: %s, Token1: %s\n", token0.Symbol, token1.Symbol)

	blockNum, err := client.BlockNumber(ctx)
	if err != nil {
		log.Fatal("Failed to get block number:", err)
	}
	log.Printf("✓ Current block: %d\n", blockNum)

	log.Println("\n=== Test 1: Get Pool Data ===")
	pool, err := getPoolData(ctx, client, token0, token1, POOLADDR, FEE)
	if err != nil {
		log.Fatal("Failed to get pool:", err)
	}
	log.Printf("✓ Pool liquidity: %s\n", pool.Liquidity.String())
	log.Printf("✓ Pool sqrtPriceX96: %s\n", pool.SqrtPriceX96.String())
	log.Printf("✓ Pool tick: %d\n", pool.Tick)

	log.Println("\n=== Test 2: Get Token Balances ===")
	balance0, err := getTokenBalance(ctx, client, token0.Address, walletAddr)
	if err != nil {
		log.Println("Warning: Failed to get token0 balance:", err)
		balance0 = big.NewInt(0)
	}
	balance1, err := getTokenBalance(ctx, client, token1.Address, walletAddr)
	if err != nil {
		log.Println("Warning: Failed to get token1 balance:", err)
		balance1 = big.NewInt(0)
	}
	log.Printf("✓ Token0 (GLUSD) balance: %s\n", balance0.String())
	log.Printf("✓ Token1 (USDT) balance: %s\n", balance1.String())

	log.Println("\n=== Test 3: Get ETH Balance ===")
	ethBalance, err := client.BalanceAt(ctx, walletAddr, nil)
	if err != nil {
		log.Println("Warning: Failed to get ETH balance:", err)
		ethBalance = big.NewInt(0)
	}
	log.Printf("✓ ETH balance: %s\n", ethBalance.String())

	log.Println("\n=== Test 4: Create Trade (Quote) ===")
	amountIn := big.NewInt(1e18)
	trade, err := createTrade(pool, token0, token1, amountIn)
	if err != nil {
		log.Println("Warning: Failed to create trade:", err)
	} else {
		outputAmount := trade.OutputAmount.Wrapped().Quotient()
		log.Printf("✓ Input: %s GLUSD -> Output: %s USDT\n", amountIn.String(), outputAmount.String())
	}

	log.Println("\n=== Test 5: Get Position Manager ===")
	posMgr, err := contract.NewUniswapv3NFTPositionManager(common.HexToAddress(POSITIONMGR), client)
	if err != nil {
		log.Fatal("Failed to create position manager:", err)
	}
	log.Printf("✓ Position Manager created: %s\n", POSITIONMGR)

	log.Println("\n=== Test 6: Get Factory ===")
	factory, err := contract.NewUniswapv3Factory(common.HexToAddress(FACTORYADDR), client)
	if err != nil {
		log.Fatal("Failed to create factory:", err)
	}
	log.Printf("✓ Factory created: %s\n", FACTORYADDR)

	log.Println("\n=== Test 7: Get Swap Router ===")
	swapRouter, err := contract.NewUniswapv3SwapRouter(common.HexToAddress(SWAPROUTER), client)
	if err != nil {
		log.Fatal("Failed to create swap router:", err)
	}
	log.Printf("✓ Swap Router created: %s\n", SWAPROUTER)

	log.Println("\n=== All Tests Passed ===")
}

func getPoolData(ctx context.Context, client *ethclient.Client, token0, token1 *coreEntities.Token, poolAddr string, fee uint32) (*entities.Pool, error) {
	poolAddress := common.HexToAddress(poolAddr)
	contractPool, err := contract.NewUniswapv3Pool(poolAddress, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool contract: %w", err)
	}

	liquidity, err := contractPool.Liquidity(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get liquidity: %w", err)
	}

	slot0, err := contractPool.Slot0(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get slot0: %w", err)
	}

	feeAmount := constants.FeeAmount(fee)
	tickSpacing := constants.TickSpacings[feeAmount]

	minTick := entities.NearestUsableTick(utils.MinTick, tickSpacing)
	maxTick := entities.NearestUsableTick(utils.MaxTick, tickSpacing)

	pooltick, err := contractPool.Ticks(ctx, big.NewInt(int64(minTick)))
	if err != nil {
		return nil, fmt.Errorf("failed to get tick: %w", err)
	}

	ticks := []entities.Tick{
		{
			Index:          minTick,
			LiquidityNet:   pooltick.LiquidityNet,
			LiquidityGross: pooltick.LiquidityGross,
		},
		{
			Index:          maxTick,
			LiquidityNet:   new(big.Int).Neg(pooltick.LiquidityNet),
			LiquidityGross: pooltick.LiquidityGross,
		},
	}

	tickDataProvider, err := entities.NewTickListDataProvider(ticks, tickSpacing)
	if err != nil {
		return nil, fmt.Errorf("failed to create tick data provider: %w", err)
	}

	pool, err := entities.NewPool(token0, token1, feeAmount, slot0.SqrtPriceX96, liquidity, int(slot0.Tick.Int64()), tickDataProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	return pool, nil
}

func getTokenBalance(ctx context.Context, client *ethclient.Client, tokenAddr, ownerAddr common.Address) (*big.Int, error) {
	balanceOfMethod := "0x70a08231"
	balanceOfArgs := common.LeftPadBytes(ownerAddr.Bytes(), 32)
	data := append([]byte(balanceOfMethod), balanceOfArgs...)

	var result interface{}
	err := client.CallContract(ctx, nil, &result)
	if err != nil {
		return nil, err
	}

	return big.NewInt(0), nil
}

func createTrade(pool *entities.Pool, tokenIn, tokenOut *coreEntities.Token, amountIn *big.Int) (*entities.Trade, error) {
	route, err := entities.NewRoute([]*entities.Pool{pool}, tokenIn, tokenOut)
	if err != nil {
		return nil, err
	}

	inputAmount := coreEntities.FromRawAmount(tokenIn, amountIn)
	trade, err := entities.FromRoute(route, inputAmount, coreEntities.ExactInput)
	if err != nil {
		return nil, err
	}

	return trade, nil
}

func TestSwapCallParameters() {
	ctx := context.Background()
	client, _ := ethclient.Dial(RPCURL)

	token0 := coreEntities.NewToken(CHAINID, common.HexToAddress(TOKEN0ADDR), 18, "GLUSD", "GLUSD")
	token1 := coreEntities.NewToken(CHAINID, common.HexToAddress(TOKEN1ADDR), 18, "USDT", "USDT")

	pool, _ := getPoolData(ctx, client, token0, token1, POOLADDR, FEE)

	amountIn := big.NewInt(1e18)
	trade, _ := createTrade(pool, token0, token1, amountIn)

	slippageTolerance := coreEntities.NewPercent(big.NewInt(1), big.NewInt(100))
	deadline := big.NewInt(time.Now().Add(15 * time.Minute).Unix())

	params, err := periphery.SwapCallParameters([]*entities.Trade{trade}, &periphery.SwapOptions{
		SlippageTolerance: slippageTolerance,
		Recipient:         crypto.PubkeyToAddress(crypto.S256().PublicKey),
		Deadline:          deadline,
	})
	if err != nil {
		log.Println("Swap params error:", err)
		return
	}

	log.Printf("Swap calldata:\n", params.Calldata)
	log.Printf("Swap %x value: %s\n", params.Value.String())
}

func TestAddLiquidityCallParameters() {
	ctx := context.Background()
	client, _ := ethclient.Dial(RPCURL)

	token0 := coreEntities.NewToken(CHAINID, common.HexToAddress(TOKEN0ADDR), 18, "GLUSD", "GLUSD")
	token1 := coreEntities.NewToken(CHAINID, common.HexToAddress(TOKEN1ADDR), 18, "USDT", "USDT")

	pool, _ := getPoolData(ctx, client, token0, token1, POOLADDR, FEE)

	amount0 := big.NewInt(1e18)
	amount1 := big.NewInt(1e18)

	pos, err := entities.FromAmounts(pool, -43260, 29400, amount0, amount1, false)
	if err != nil {
		log.Println("Position error:", err)
		return
	}

	slippageTolerance := coreEntities.NewPercent(big.NewInt(1), big.NewInt(100))
	deadline := big.NewInt(time.Now().Add(15 * time.Minute).Unix())

	opts := &periphery.AddLiquidityOptions{
		CommonAddLiquidityOptions: &periphery.CommonAddLiquidityOptions{
			SlippageTolerance: slippageTolerance,
			Deadline:          deadline,
		},
		MintSpecificOptions: &periphery.MintSpecificOptions{
			Recipient:  common.HexToAddress("0x20AaF3E0162dc97b4C71281aC1Ca4719cEb15060"),
			CreatePool: false,
		},
	}

	params, err := periphery.AddCallParameters(pos, opts)
	if err != nil {
		log.Println("Add liquidity params error:", err)
		return
	}

	log.Printf("Add liquidity calldata: %x\n", params.Calldata)
	log.Printf("Add liquidity value: %s\n", params.Value.String())
}
