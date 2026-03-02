// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// Simple reentrancy guard
contract ReentrancyGuard {
    uint256 private _status = 1;
    modifier nonReentrant() {
        require(_status == 1, "ReentrancyGuard: reentrant call");
        _status = 2;
        _;
        _status = 1;
    }
}

// Ownable minimal
abstract contract Ownable {
    address public owner;
    constructor(address _owner) { owner = _owner; }
    modifier onlyOwner() { require(msg.sender == owner); _; }
}

// ERC20 interface
interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
}
interface IERC20Metadata is IERC20 {
    function decimals() external view returns (uint8);
}

// SafeERC20 minimal
library SafeERC20 {
    function safeTransfer(IERC20 token, address to, uint256 amount) internal {
        (bool success, bytes memory data) = address(token).call(abi.encodeCall(IERC20.transfer, (to, amount)));
        require(success && (data.length == 0 || abi.decode(data, (bool))));
    }
    function safeTransferFrom(IERC20 token, address from, address to, uint256 amount) internal {
        (bool success, bytes memory data) = address(token).call(abi.encodeCall(IERC20.transferFrom, (from, to, amount)));
        require(success && (data.length == 0 || abi.decode(data, (bool))));
    }
    function forceApprove(IERC20 token, address spender, uint256 value) internal {
        (bool success, bytes memory data) = address(token).call(abi.encodeCall(IERC20.approve, (spender, value)));
        require(success && (data.length == 0 || abi.decode(data, (bool))));
    }
}

// Uniswap libraries
import "@uniswap/v3-core/contracts/libraries/TickMath.sol";
import "@uniswap/v3-core/contracts/libraries/TickBitmap.sol";
import "@uniswap/v3-core/contracts/libraries/SwapMath.sol";
import "@uniswap/v3-core/contracts/libraries/LiquidityMath.sol";

// Uniswap interfaces
interface IUniswapV3Pool {
    function slot0() external view returns (uint160 sqrtPriceX96, int24 tick, uint16, uint16, uint16, uint8, bool);
    function liquidity() external view returns (uint128);
    function tickSpacing() external view returns (int24);
    function tickBitmap(int16) external view returns (uint256);
    function ticks(int24) external view returns (uint128, int128, uint256, uint256, int56, uint160, uint32, bool);
}
interface IUniswapV3Factory {
    function getPool(address, address, uint24) external view returns (address);
}
interface ISwapRouter {
    struct ExactInputSingleParams {
        address tokenIn; address tokenOut; uint24 fee; address recipient;
        uint256 deadline; uint256 amountIn; uint256 amountOutMinimum; uint160 sqrtPriceLimitX96;
    }
    function exactInputSingle(ExactInputSingleParams calldata params) external payable returns (uint256);
}

contract StabilizationVault is Ownable, ReentrancyGuard {
    using SafeERC20 for IERC20;

    address public token0; address public token1;
    uint256 public reserve0; uint256 public reserve1;
    IUniswapV3Pool public pool; ISwapRouter public swapRouter;
    uint24 public fee; uint256 public rebalanceThresholdBps = 20;
    uint256 public targetPrice;
    bool public circuitBreakerActive = false;
    uint256 public lastExecutionTime;
    address public factory;

    event Deposit(address,uint256,uint256);
    event Withdraw(address,uint256,uint256);
    event ArbitrageExecuted(uint256,uint256,bool);
    event CircuitBreakerTriggered(string);
    event ParametersUpdated(string,uint256);

    constructor(address _token0, address _token1, uint24 _fee, address _factory, address _swapRouter, address _owner) 
        Ownable(_owner) {
        require(_token0 != address(0) && _token1 != address(0) && _factory != address(0));
        token0 = _token0; token1 = _token1; factory = _factory;
        pool = IUniswapV3Pool(IUniswapV3Factory(_factory).getPool(token0, token1, _fee));
        require(address(pool) != address(0));
        fee = _fee; swapRouter = ISwapRouter(_swapRouter); targetPrice = 1;
    }

    function deposit(uint256 amount0, uint256 amount1) external onlyOwner nonReentrant {
        require(amount0 > 0 || amount1 > 0);
        if (amount0 > 0) IERC20(token0).safeTransferFrom(msg.sender, address(this), amount0);
        if (amount1 > 0) IERC20(token1).safeTransferFrom(msg.sender, address(this), amount1);
        reserve0 += amount0; reserve1 += amount1;
        emit Deposit(msg.sender, amount0, amount1);
    }

    function withdraw(uint256 amount0, uint256 amount1) external onlyOwner nonReentrant {
        require(amount0 > 0 || amount1 > 0);
        require(amount0 <= reserve0 && amount1 <= reserve1);
        reserve0 -= amount0; reserve1 -= amount1;
        if (amount0 > 0) IERC20(token0).safeTransfer(msg.sender, amount0);
        if (amount1 > 0) IERC20(token1).safeTransfer(msg.sender, amount1);
        emit Withdraw(msg.sender, amount0, amount1);
    }

    function getPrice() public view returns (uint160 sqrtPriceX96, int24 tick) {
        (sqrtPriceX96, tick, , , , , ) = pool.slot0();
    }
    function getLiquidity() public view returns (uint128) { return pool.liquidity(); }

    function calculateSwapAmount(uint256 _targetPrice) public view returns (uint256 amountIn) {
        (uint160 sqrtPriceX96, int24 tick, , , , , ) = pool.slot0();
        uint128 liquidity = pool.liquidity();
        require(liquidity > 0, "No liquidity");
        uint160 targetSqrtPriceX96 = _priceToSqrtPriceX96(_targetPrice);
        (amountIn, , ) = amountInToReachTarget(pool, targetSqrtPriceX96);
        uint256 minAmount = 10 ** IERC20Metadata(token0).decimals();
        if (amountIn < minAmount) amountIn = minAmount;
    }

    function amountInToReachTarget(IUniswapV3Pool pool, uint160 targetSqrtPriceX96) public view 
        returns (uint256 amountInWithFee, int24 finalTick, uint128 finalLiquidity) {
        (uint160 sqrtPriceX96, int24 tick, , , , , ) = pool.slot0();
        uint128 liquidity = pool.liquidity();
        int24 tickSpacing = pool.tickSpacing();
        require(liquidity > 0);
        require(targetSqrtPriceX96 != sqrtPriceX96);
        bool zeroForOne = sqrtPriceX96 > targetSqrtPriceX96;
        while (sqrtPriceX96 != targetSqrtPriceX96) {
            (int24 nextTick, ) = _nextInitializedTick(pool, tick, tickSpacing, zeroForOne);
            if (nextTick < TickMath.MIN_TICK) nextTick = TickMath.MIN_TICK;
            if (nextTick > TickMath.MAX_TICK) nextTick = TickMath.MAX_TICK;
            uint160 sqrtPriceNextTickX96 = TickMath.getSqrtRatioAtTick(nextTick);
            uint160 stepTarget = zeroForOne 
                ? (sqrtPriceNextTickX96 < targetSqrtPriceX96 ? targetSqrtPriceX96 : sqrtPriceNextTickX96)
                : (sqrtPriceNextTickX96 > targetSqrtPriceX96 ? targetSqrtPriceX96 : sqrtPriceNextTickX96);
            (uint256 amountIn, , uint160 sqrtPriceNextX96) = SwapMath.computeSwapStep(
                sqrtPriceX96, stepTarget, liquidity,
                zeroForOne ? type(int256).max : 0, zeroForOne ? int256(fee) : 0, zeroForOne ? 0 : type(uint256).max);
            amountInWithFee += amountIn;
            sqrtPriceX96 = sqrtPriceNextX96;
            if (sqrtPriceX96 == sqrtPriceNextTickX96) {
                (, int128 liquidityNet, , , , , , ) = pool.ticks(nextTick);
                liquidity = LiquidityMath.addDelta(liquidity, liquidityNet);
                tick = zeroForOne ? nextTick - 1 : nextTick;
            } else {
                tick = TickMath.getTickAtSqrtRatio(sqrtPriceX96);
            }
            if (sqrtPriceX96 == targetSqrtPriceX96) break;
        }
        finalTick = tick; finalLiquidity = liquidity;
    }

    function _nextInitializedTick(IUniswapV3Pool pool, int24 tick, int24 ts, bool zf1) internal view returns (int24, bool) {
        int24 compressed = tick / ts;
        if (tick < 0 && tick % ts != 0) compressed--;
        return _nextInitializedTickWithinOneWord(pool, compressed, ts, zf1);
    }
    function _nextInitializedTickWithinOneWord(IUniswapV3Pool pool, int24 ct, int24 ts, bool lte) internal view returns (int24, bool) {
        (int16 wp, uint8 bp) = TickBitmap.position(ct);
        uint256 w = pool.tickBitmap(wp);
        (int24 nc, bool i) = TickBitmap.nextInitializedTickWithinOneWord(w, ct, ts, lte);
        return (nc, i);
    }

    function _priceToSqrtPriceX96(uint256 p) internal pure returns (uint160) {
        return TickMath.getSqrtRatioAtTick(_priceToTick(p));
    }
    function _priceToTick(uint256 p) internal pure returns (int24) {
        return TickMath.getTickAtSqrtRatio(TickMath.getSqrtRatioAtTick(p));
    }

    function executeArbitrage() external onlyOwner nonReentrant returns (bool) {
        require(!circuitBreakerActive);
        (uint160 sqrtPriceX96, ) = getPrice();
        uint256 currentPrice = (uint256(sqrtPriceX96) ** 2) >> 192;
        uint256 priceDiff = currentPrice > targetPrice ? currentPrice - targetPrice : targetPrice - currentPrice;
        uint256 deviationBps = (priceDiff * 10000) / targetPrice;
        require(deviationBps >= rebalanceThresholdBps, "Price deviation below threshold");
        bool zeroForOne = currentPrice > targetPrice;
        address tokenIn = zeroForOne ? token0 : token1;
        address tokenOut = zeroForOne ? token1 : token0;
        uint256 amountIn = calculateSwapAmount(targetPrice);
        require(amountIn > 0);
        uint256 balBefore = IERC20(tokenIn).balanceOf(address(this));
        require(balBefore >= amountIn, "Insufficient balance");
        IERC20(tokenIn).forceApprove(address(swapRouter), amountIn);
        uint256 amountOut = ISwapRouter(address(swapRouter)).exactInputSingle(ISwapRouter.ExactInputSingleParams({
            tokenIn: tokenIn, tokenOut: tokenOut, fee: fee, recipient: address(this),
            deadline: block.timestamp, amountIn: amountIn, amountOutMinimum: 0, sqrtPriceLimitX96: 0
        }));
        uint256 balInAfter = IERC20(tokenIn).balanceOf(address(this));
        uint256 balOutAfter = IERC20(tokenOut).balanceOf(address(this));
        require(amountOut > 0, "No output");
        if (zeroForOne) { reserve0 = balInAfter; reserve1 = balOutAfter; }
        else { reserve0 = balOutAfter; reserve1 = balInAfter; }
        lastExecutionTime = block.timestamp;
        emit ArbitrageExecuted(amountIn, amountOut, true);
        return true;
    }

    function triggerCircuitBreaker(string calldata reason) external onlyOwner {
        circuitBreakerActive = true;
        emit CircuitBreakerTriggered(reason);
    }
    function releaseCircuitBreaker() external onlyOwner { circuitBreakerActive = false; }
    function setRebalanceThresholdBps(uint256 _t) external onlyOwner {
        require(_t <= 1000);
        rebalanceThresholdBps = _t;
        emit ParametersUpdated("rebalanceThresholdBps", _t);
    }
    function setTargetPrice(uint256 _p) external onlyOwner { targetPrice = _p; emit ParametersUpdated("targetPrice", _p); }
    function getPriceDeviation() external view returns (uint256 deviationBps, bool aboveThreshold) {
        (uint160 sqrtPriceX96, ) = getPrice();
        uint256 currentPrice = (uint256(sqrtPriceX96) ** 2) >> 192;
        uint256 priceDiff = currentPrice > targetPrice ? currentPrice - targetPrice : targetPrice - currentPrice;
        deviationBps = (priceDiff * 10000) / targetPrice;
        aboveThreshold = deviationBps >= rebalanceThresholdBps;
    }
    function getReserves() external view returns (uint256, uint256) { return (reserve0, reserve1); }
    function getVaultTVL() external view returns (uint256) { return reserve0 + reserve1; }
    function emergencyWithdraw(address t, address to, uint256 a) external onlyOwner {
        require(to != address(0));
        IERC20(t).safeTransfer(to, a);
    }
}
