package executor

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"uniswap-bot/config"
	"uniswap-bot/pkg/contracts"
)

type ExecutionResult struct {
	Success        bool
	TxHash         string
	Error          error
	ErrorCode      string
	GasUsed        uint64
	Timestamp      time.Time
	TokenID        *big.Int
	Amount0        *big.Int
	Amount1        *big.Int
	PoolAddress    string
	BalanceChange0 *big.Int
	BalanceChange1 *big.Int
}

type Executor struct {
	cfg        *config.Config
	ethClient  *ethclient.Client
	chainID    *big.Int
	privateKey *ecdsa.PrivateKey
	maxRetries int
	walletAddress common.Address

	factory     *contracts.Uniswapv3Factory
	positionMgr *contracts.Uniswapv3NFTPositionManager
	swapRouter  *contracts.Uniswapv3RouterV2
	quoter      *contracts.Uniswapv3Quoter
	pool        *contracts.Uniswapv3Pool
}

func NewExecutor(cfg *config.Config) (*Executor, error) {
	client, err := ethclient.Dial(cfg.Uniswap.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ethclient: %w", err)
	}

	privateKey, err := crypto.HexToECDSA(cfg.Bot.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	walletAddress := crypto.PubkeyToAddress(privateKey.PublicKey)

	factory, err := contracts.NewUniswapv3Factory(common.HexToAddress(cfg.Uniswap.FactoryAddress), client)
	if err != nil {
		return nil, fmt.Errorf("failed to create factory: %w", err)
	}

	positionMgr, err := contracts.NewUniswapv3NFTPositionManager(common.HexToAddress(cfg.Uniswap.PositionManager), client)
	if err != nil {
		return nil, fmt.Errorf("failed to create position manager: %w", err)
	}

	swapRouter, err := contracts.NewUniswapv3RouterV2(common.HexToAddress(cfg.Uniswap.SwapRouter), client)
	if err != nil {
		return nil, fmt.Errorf("failed to create swap router: %w", err)
	}

	quoter, err := contracts.NewUniswapv3Quoter(common.HexToAddress(cfg.Uniswap.Quoter), client)
	if err != nil {
		return nil, fmt.Errorf("failed to create quoter: %w", err)
	}

	pool, err := contracts.NewUniswapv3Pool(common.HexToAddress(cfg.Uniswap.PoolAddress), client)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	return &Executor{
		cfg:           cfg,
		ethClient:     client,
		chainID:       big.NewInt(cfg.Uniswap.ChainID),
		privateKey:    privateKey,
		maxRetries:    cfg.Execution.RetryTimes,
		walletAddress: walletAddress,
		factory:       factory,
		positionMgr:   positionMgr,
		swapRouter:    swapRouter,
		quoter:        quoter,
		pool:          pool,
	}, nil
}

func (e *Executor) GetWalletAddress() common.Address {
	return e.walletAddress
}

func (e *Executor) GetTokenBalance(ctx context.Context, tokenAddr, ownerAddr common.Address) (*big.Int, error) {
	return GetTokenBalance(ctx, e.ethClient, tokenAddr, ownerAddr)
}

func (e *Executor) GetEthBalance(ctx context.Context, address common.Address) (*big.Int, error) {
	return e.ethClient.BalanceAt(ctx, address, nil)
}

func (e *Executor) QueryPoolAddress(ctx context.Context, token0, token1 common.Address, fee uint32) (common.Address, error) {
	return e.factory.GetPool(nil, token0, token1, big.NewInt(int64(fee)))
}

func (e *Executor) CreatePool(ctx context.Context, token0, token1 common.Address, fee uint32) (*ExecutionResult, error) {
	existingPool, err := e.QueryPoolAddress(ctx, token0, token1, fee)
	if err == nil && existingPool != (common.Address{}) {
		return &ExecutionResult{
			Success:     true,
			PoolAddress: existingPool.Hex(),
			Timestamp:   time.Now(),
		}, nil
	}

	auth, err := bind.NewKeyedTransactorWithChainID(e.privateKey, e.chainID)
	if err != nil {
		return &ExecutionResult{Success: false, Error: err, Timestamp: time.Now()}, err
	}

	tx, err := e.factory.CreatePool(auth, token0, token1, big.NewInt(int64(fee)))
	if err != nil {
		return &ExecutionResult{Success: false, Error: err, Timestamp: time.Now()}, err
	}

	receipt, err := waitForReceipt(ctx, e.ethClient, tx)
	if err != nil {
		return &ExecutionResult{Success: false, TxHash: tx.Hash().Hex(), Error: err, Timestamp: time.Now()}, err
	}

	poolAddress, _ := e.factory.GetPool(nil, token0, token1, big.NewInt(int64(fee)))

	return &ExecutionResult{
		Success:     receipt.Status == types.ReceiptStatusSuccessful,
		TxHash:      tx.Hash().Hex(),
		GasUsed:     receipt.GasUsed,
		PoolAddress: poolAddress.Hex(),
		Timestamp:   time.Now(),
	}, nil
}

func (e *Executor) AddLiquidity(ctx context.Context, token0, token1 common.Address, fee uint32, amount0, amount1 *big.Int, tickLower, tickUpper int32) (*ExecutionResult, error) {
	positionMgrAddr := common.HexToAddress(e.cfg.Uniswap.PositionManager)

	log.Printf("=== AddLiquidity ===")
	log.Printf("Position Manager: %s", positionMgrAddr.Hex())
	log.Printf("Token0: %s", token0.Hex())
	log.Printf("Token1: %s", token1.Hex())
	log.Printf("Amount0: %s", amount0.String())
	log.Printf("Amount1: %s", amount1.String())
	log.Printf("TickLower: %d", tickLower)
	log.Printf("TickUpper: %d", tickUpper)

	if err := e.ApproveToken(ctx, token0, positionMgrAddr, amount0); err != nil {
		log.Printf("Approve token0 failed: %v", err)
		return &ExecutionResult{Success: false, Error: fmt.Errorf("approve token0 failed: %w", err), Timestamp: time.Now()}, err
	}

	if err := e.ApproveToken(ctx, token1, positionMgrAddr, amount1); err != nil {
		log.Printf("Approve token1 failed: %v", err)
		return &ExecutionResult{Success: false, Error: fmt.Errorf("approve token1 failed: %w", err), Timestamp: time.Now()}, err
	}

	log.Printf("All approvals done, now minting...")

	auth, err := bind.NewKeyedTransactorWithChainID(e.privateKey, e.chainID)
	if err != nil {
		return &ExecutionResult{Success: false, Error: err, Timestamp: time.Now()}, err
	}

	params := contracts.INonfungiblePositionManagerMintParams{
		Token0:         token0,
		Token1:         token1,
		Fee:            big.NewInt(int64(fee)),
		TickLower:      big.NewInt(int64(tickLower)),
		TickUpper:      big.NewInt(int64(tickUpper)),
		Amount0Desired: amount0,
		Amount1Desired: amount1,
		Amount0Min:     big.NewInt(0),
		Amount1Min:     big.NewInt(0),
		Recipient:      e.walletAddress,
		Deadline:       big.NewInt(time.Now().Unix() + 300),
	}

	log.Printf("mint params: ", params)
	tx, err := e.positionMgr.Mint(auth, params)
	if err != nil {
		return &ExecutionResult{Success: false, Error: err, Timestamp: time.Now()}, err
	}

	receipt, err := waitForReceipt(ctx, e.ethClient, tx)
	if err != nil {
		return &ExecutionResult{Success: false, TxHash: tx.Hash().Hex(), Error: err, Timestamp: time.Now()}, err
	}

	var tokenID *big.Int
	for _, rlog := range receipt.Logs {
		if len(rlog.Data) >= 96 {
			tokenID = new(big.Int).SetBytes(rlog.Data[0:32])
			break
		}
	}

	return &ExecutionResult{
		Success:   receipt.Status == types.ReceiptStatusSuccessful,
		TxHash:    tx.Hash().Hex(),
		GasUsed:   receipt.GasUsed,
		TokenID:   tokenID,
		Timestamp: time.Now(),
	}, nil
}

func (e *Executor) ApproveToken(ctx context.Context, tokenAddr, spender common.Address, amount *big.Int) error {
	erc20, err := NewERC20(tokenAddr, e.ethClient)
	if err != nil {
		return err
	}

	allowance, err := erc20.Allowance(ctx, e.walletAddress, spender)
	if err != nil {
		log.Printf("Allowance check failed: %v", err)
		return err
	}
	log.Printf("Current allowance for %s: %s", tokenAddr.Hex(), allowance.String())

	if allowance.Cmp(amount) >= 0 {
		log.Printf("Allowance sufficient, skipping approve")
		return nil
	}

	log.Printf("Approving %s to spend %s...", spender.Hex(), amount.String())
	tx, err := erc20.Approve(ctx, e.privateKey, spender, big.NewInt(0).Mul(amount, big.NewInt(2)), e.chainID.Int64())
	if err != nil {
		log.Printf("Approve failed: %v", err)
		return err
	}

	log.Printf("Approve tx sent: %s", tx.Hash().Hex())

	receipt, err := waitForReceipt(ctx, e.ethClient, tx)
	if err != nil {
		log.Printf("Approve receipt error: %v", err)
		return err
	}

	log.Printf("Approve tx status: %d", receipt.Status)
	return nil
}

func (e *Executor) ExecuteSwap(ctx context.Context, tokenIn, tokenOut common.Address, amountIn *big.Int, amountOutMin *big.Int, sqrtPriceLimitX96 *big.Int) (*ExecutionResult, error) {
	auth, err := bind.NewKeyedTransactorWithChainID(e.privateKey, e.chainID)
	if err != nil {
		return &ExecutionResult{Success: false, Error: err, Timestamp: time.Now()}, err
	}

	params := contracts.IV3SwapRouterExactInputSingleParams{
		TokenIn:           tokenIn,
		TokenOut:          tokenOut,
		Fee:               big.NewInt(int64(e.cfg.Uniswap.FeeTier)),
		Recipient:         e.walletAddress,
		AmountIn:          amountIn,
		AmountOutMinimum:  amountOutMin,
		SqrtPriceLimitX96: sqrtPriceLimitX96,
	}

	tx, err := e.swapRouter.ExactInputSingle(auth, params)
	if err != nil {
		return &ExecutionResult{Success: false, Error: err, Timestamp: time.Now()}, err
	}

	receipt, err := waitForReceipt(ctx, e.ethClient, tx)
	if err != nil {
		return &ExecutionResult{Success: false, TxHash: tx.Hash().Hex(), Error: err, Timestamp: time.Now()}, err
	}

	return &ExecutionResult{
		Success:   receipt.Status == types.ReceiptStatusSuccessful,
		TxHash:    tx.Hash().Hex(),
		GasUsed:   receipt.GasUsed,
		Timestamp: time.Now(),
	}, nil
}

func (e *Executor) QuoteSwap(ctx context.Context, tokenIn, tokenOut common.Address, amountIn *big.Int) (*big.Int, error) {
	result, err := e.quoter.QuoteExactInputSingle(nil, tokenIn, tokenOut, big.NewInt(int64(e.cfg.Uniswap.FeeTier)), amountIn, big.NewInt(0))
	if err != nil {
		return nil, err
	}

	receipt, err := waitForReceipt(ctx, e.ethClient, result)
	if err != nil {
		return nil, err
	}

	var amountOut *big.Int
	for _, rlog := range receipt.Logs {
		if len(rlog.Data) >= 32 {
			amountOut = new(big.Int).SetBytes(rlog.Data)
			break
		}
	}

	return amountOut, nil
}

func (e *Executor) GetPosition(ctx context.Context, tokenID *big.Int) (struct {
	Nonce                    *big.Int
	Operator                 common.Address
	Token0                   common.Address
	Token1                   common.Address
	Fee                      *big.Int
	TickLower                *big.Int
	TickUpper                *big.Int
	Liquidity                *big.Int
	FeeGrowthInside0LastX128 *big.Int
	FeeGrowthInside1LastX128 *big.Int
	TokensOwed0              *big.Int
	TokensOwed1              *big.Int
}, error) {
	return e.positionMgr.Positions(nil, tokenID)
}

func (e *Executor) GetFactory() *contracts.Uniswapv3Factory {
	return e.factory
}

func (e *Executor) GetPositionManager() *contracts.Uniswapv3NFTPositionManager {
	return e.positionMgr
}

func (e *Executor) GetSwapRouter() *contracts.Uniswapv3RouterV2 {
	return e.swapRouter
}

func (e *Executor) GetChainID() int64 {
	return e.cfg.Uniswap.ChainID
}

func (e *Executor) Close() {
	e.ethClient.Close()
}

func waitForReceipt(ctx context.Context, client *ethclient.Client, tx *types.Transaction) (*types.Receipt, error) {
	for {
		receipt, err := client.TransactionReceipt(ctx, tx.Hash())
		if err != nil {
			if err == ethereum.NotFound {
				time.Sleep(2 * time.Second)
				continue
			}
			return nil, err
		}
		return receipt, nil
	}
}

func PriceToTick(price float64) int32 {
	if price <= 0 {
		return 0
	}
	sqrtPrice := math.Sqrt(price)
	tick := math.Log(sqrtPrice) / math.Log(1.0001)
	return int32(math.Floor(tick))
}

func TickToPrice(tick int32) float64 {
	return 1.0
}

func uint24(v uint32) *big.Int {
	return big.NewInt(int64(v))
}

func GetTickSpacing(fee uint32) int32 {
	switch fee {
	case 100:
		return 1
	case 500:
		return 10
	case 3000:
		return 60
	case 10000:
		return 200
	default:
		return 10
	}
}

func AlignTickToSpacing(tick int32, spacing int32) int32 {
	if tick < 0 {
		return (tick / spacing) * spacing
	}
	return (tick / spacing) * spacing
}

func CalculateTickRange(refPrice float64, fee uint32, rangeBps int) (int32, int32) {
	lower := refPrice * (1 - float64(rangeBps)/10000)
	upper := refPrice * (1 + float64(rangeBps)/10000)
	tickLower := PriceToTick(lower)
	tickUpper := PriceToTick(upper)
	spacing := GetTickSpacing(fee)
	tickLower = AlignTickToSpacing(tickLower, spacing)
	tickUpper = AlignTickToSpacing(tickUpper, spacing)
	return tickLower, tickUpper
}

type TierPosition struct {
	Name      string   `json:"name"`
	Amount0   *big.Int `json:"amount0"`
	Amount1   *big.Int `json:"amount1"`
}

func (e *Executor) GetTierPositions(ctx context.Context) ([]TierPosition, error) {
	walletAddr := e.walletAddress

	balance, err := e.positionMgr.BalanceOf(&bind.CallOpts{Context: ctx}, walletAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	coreAmount0 := big.NewInt(0)
	coreAmount1 := big.NewInt(0)
	midAmount0 := big.NewInt(0)
	midAmount1 := big.NewInt(0)
	tailAmount0 := big.NewInt(0)
	tailAmount1 := big.NewInt(0)

	slot0, err := e.pool.Slot0(&bind.CallOpts{Context: ctx})
	if err != nil {
		return nil, fmt.Errorf("failed to get slot0: %w", err)
	}

	sqrtPriceX96 := slot0.SqrtPriceX96

	for i := uint64(0); i < balance.Uint64(); i++ {
		tokenId, err := e.positionMgr.TokenOfOwnerByIndex(&bind.CallOpts{Context: ctx}, walletAddr, big.NewInt(int64(i)))
		if err != nil {
			continue
		}

		pos, err := e.positionMgr.Positions(&bind.CallOpts{Context: ctx}, tokenId)
		if err != nil {
			continue
		}

		tickLower := int32(pos.TickLower.Int64())
		tickUpper := int32(pos.TickUpper.Int64())
		liquidity := pos.Liquidity

		if liquidity.Sign() == 0 {
			continue
		}

		amount0, amount1 := e.calculateTokenAmounts(liquidity, tickLower, tickUpper, sqrtPriceX96)

		if tickLower >= -10 && tickUpper <= 10 {
			coreAmount0.Add(coreAmount0, amount0)
			coreAmount1.Add(coreAmount1, amount1)
		} else if tickLower >= -50 && tickUpper <= 50 {
			midAmount0.Add(midAmount0, amount0)
			midAmount1.Add(midAmount1, amount1)
		} else if tickLower >= -200 && tickUpper <= 200 {
			tailAmount0.Add(tailAmount0, amount0)
			tailAmount1.Add(tailAmount1, amount1)
		}
	}

	return []TierPosition{
		{Name: "core", Amount0: coreAmount0, Amount1: coreAmount1},
		{Name: "mid", Amount0: midAmount0, Amount1: midAmount1},
		{Name: "tail", Amount0: tailAmount0, Amount1: tailAmount1},
	}, nil
}

func (e *Executor) calculateTokenAmounts(liquidity *big.Int, tickLower, tickUpper int32, sqrtPriceX96 *big.Int) (*big.Int, *big.Int) {
	sqrtRatioX96 := sqrtPriceX96

	sqrtLower := e.tickToSqrtRatioX96(tickLower)
	sqrtUpper := e.tickToSqrtRatioX96(tickUpper)

	oneX96 := new(big.Int).Lsh(big.NewInt(1), 96)

	if liquidity.Sign() == 0 {
		return big.NewInt(0), big.NewInt(0)
	}

	if sqrtLower.Sign() == 0 || sqrtUpper.Sign() == 0 || sqrtRatioX96.Sign() == 0 {
		return big.NewInt(0), big.NewInt(0)
	}

	currentTick := int32(0)
	if sqrtRatioX96.Sign() > 0 {
		currentTick = int32(math.Floor(math.Log(float64(sqrtRatioX96.Uint64())/float64(1<<64)) / math.Log(1.0001)))
	}

	var amount0, amount1 *big.Int

	if currentTick <= tickLower {
		amount0 = new(big.Int).Mul(liquidity, new(big.Int).Sub(sqrtUpper, sqrtLower))
		amount0 = new(big.Int).Div(amount0, sqrtLower)
		amount0 = new(big.Int).Div(amount0, oneX96)
		amount1 = big.NewInt(0)
	} else if currentTick >= tickUpper {
		amount0 = big.NewInt(0)
		amount1 = new(big.Int).Mul(liquidity, new(big.Int).Sub(sqrtUpper, sqrtLower))
		amount1 = new(big.Int).Div(amount1, sqrtUpper)
		amount1 = new(big.Int).Div(amount1, oneX96)
	} else {
		amount0 = new(big.Int).Mul(liquidity, new(big.Int).Sub(sqrtRatioX96, sqrtLower))
		amount0 = new(big.Int).Div(amount0, sqrtLower)
		amount0 = new(big.Int).Div(amount0, oneX96)

		amount1 = new(big.Int).Mul(liquidity, new(big.Int).Sub(sqrtUpper, sqrtRatioX96))
		amount1 = new(big.Int).Div(amount1, sqrtUpper)
		amount1 = new(big.Int).Div(amount1, oneX96)
	}

	return amount0, amount1
}

func (e *Executor) tickToSqrtRatioX96(tick int32) *big.Int {
	absTick := tick
	if absTick < 0 {
		absTick = -absTick
	}

	ratio := new(big.Int).Lsh(big.NewInt(1), 96)

	pos := absTick / 2
	for i := int32(0); i < pos; i++ {
		tmp1, _ := new(big.Int).SetString("79228162514264337593543950336", 10)
		tmp2, _ := new(big.Int).SetString("999500001833390168889799", 10)
		ratio = new(big.Int).Mul(ratio, tmp1)
		ratio = new(big.Int).Div(ratio, tmp2)
	}

	if absTick%2 == 1 {
		tmp1, _ := new(big.Int).SetString("79228162514264337593543950336", 10)
		tmp2, _ := new(big.Int).SetString("999500001833390168889799", 10)
		ratio = new(big.Int).Mul(ratio, tmp1)
		ratio = new(big.Int).Div(ratio, tmp2)
	}

	if tick < 0 {
		ratio = new(big.Int).Div(new(big.Int).Lsh(big.NewInt(1), 96), ratio)
	}

	return ratio
}
