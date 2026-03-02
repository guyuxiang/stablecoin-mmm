// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract ReentrancyGuard {
    uint256 private _status = 1;
    modifier nonReentrant() { require(_status == 1); _status = 2; _; _status = 1; }
}
abstract contract Ownable { address public owner; constructor(address _owner) { owner = _owner; } modifier onlyOwner() { require(msg.sender == owner); _; } }
interface IERC20 { function transfer(address to, uint256 amount) external returns (bool); function transferFrom(address from, address to, uint256 amount) external returns (bool); function balanceOf(address account) external view returns (uint256); function approve(address spender, uint256 amount) external returns (bool); }
interface IERC20Metadata is IERC20 { function decimals() external view returns (uint8); }
library SafeERC20 {
    function safeTransfer(IERC20 t, address to, uint256 a) internal { (bool s,) = address(t).call(abi.encodeCall(IERC20.transfer, (to, a))); require(s); }
    function safeTransferFrom(IERC20 t, address f, address to, uint256 a) internal { (bool s,) = address(t).call(abi.encodeCall(IERC20.transferFrom, (f, to, a))); require(s); }
    function forceApprove(IERC20 t, address s, uint256 v) internal { (bool s,) = address(t).call(abi.encodeCall(IERC20.approve, (s, v))); require(s); }
}
library TickMath { int24 constant MIN_TICK = -887272; int24 constant MAX_TICK = 887272;
    function getSqrtRatioAtTick(int24 tick) internal pure returns (uint160) {
        uint256 absTick = tick < 0 ? uint256(-tick) : uint256(tick);
        uint256 ratio = 0x100000000000000000000000000000000;
        if (absTick & 0x1 != 0) ratio = 0xfffcb933bd6afaeb90559b993b71e22a3 * ratio / 0x100000000000000000000000000000000;
        if (absTick & 0x2 != 0) ratio = 0xfff97272373d413259a46990580e213a * ratio / 0x100000000000000000000000000000000;
        if (absTick & 0x4 != 0) ratio = 0xfff2e50f5f656932ef12357cf3c7fdcc * ratio / 0x100000000000000000000000000000000;
        if (absTick & 0x8 != 0) ratio = 0xffe5caca7e10e4e61c3624eaa0941cd0 * ratio / 0x100000000000000000000000000000000;
        if (absTick & 0x10 != 0) ratio = 0xffcbb2cbda5ce6bc340bab24e2e63e8e * ratio / 0x100000000000000000000000000000000;
        if (tick < 0) ratio = 0x100000000000000000000000000000000 / ratio;
        return uint160(ratio * 0x10000000000000000 / 2**64);
    }
    function getTickAtSqrtRatio(uint160 sqrtPriceX96) internal pure returns (int24) {
        return int24((sqrtPriceX96 / 0x1000000000000) - 2**23);
    }
}
library TickBitmap {
    function position(int24 tick) internal pure returns (int16 wordPos, uint8 bitPos) { wordPos = int16(tick >> 8); bitPos = uint8(tick % 256); }
    function nextInitializedTickWithinOneWord(uint256 bitmap, int24 tick, int24 tickSpacing, bool lte) internal pure returns (int24 next, bool initialized) {
        int24 compressed = tick / tickSpacing;
        if (tick < 0 && tick % tickSpacing != 0) compressed--;
        if (lte) {
            uint256 mask = (1 << (compressed % 256 + 1)) - 1;
            uint256 masked = bitmap & mask;
            initialized = masked != 0;
            next = initialized ? compressed - int24(ffs(masked) - 1) : compressed - int24(256);
        } else {
            uint256 mask = ~((1 << compressed % 256) - 1);
            uint256 masked = bitmap & mask;
            initialized = masked != 0;
            next = initialized ? compressed + 1 + int24(ffs(masked >> 1) - 1) : compressed + 1 + 256;
        }
    }
    function ffs(uint256 x) internal pure returns (uint256 r) {
        assembly { r := shl(7, gt(x, 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF)) 
                  r := or(r, shl(6, gt(and(x, 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF), 0xFFFFFFFFFFFFFFFFFFFFFFFF))) }
    }
}
library SwapMath {
    function computeSwapStep(uint160 sqrtPriceCurrentX96, uint160 sqrtPriceTargetX96, uint128 liquidity, int256 amountRemaining, int256 feeAmount, uint256 sqrtPriceLimitX96) internal pure returns (uint256 amountIn, uint256 amountOut, uint160 sqrtPriceNextX96) {
        bool zeroForOne = sqrtPriceCurrentX96 > sqrtPriceTargetX96;
        amountIn = zeroForOne ? uint256(-amountRemaining) : 0;
        amountOut = zeroForOne ? 0 : uint256(amountRemaining);
        sqrtPriceNextX96 = sqrtPriceTargetX96;
    }
}
library LiquidityMath { function addDelta(uint128 x, int128 y) internal pure returns (uint128 z) { if (y < 0) z = x - uint128(uint128(-y)); else z = x + uint128(uint128(y)); } }
interface IUniswapV3Pool { function slot0() external view returns (uint160, int24, uint16, uint16, uint16, uint8, bool); function liquidity() external view returns (uint128); function tickSpacing() external view returns (int24); function tickBitmap(int16) external view returns (uint256); function ticks(int24) external view returns (uint128, int128, uint256, uint256, int56, uint160, uint32, bool); }
interface IUniswapV3Factory { function getPool(address, address, uint24) external view returns (address); }
interface ISwapRouter { struct ExactInputSingleParams { address tokenIn; address tokenOut; uint24 fee; address recipient; uint256 deadline; uint256 amountIn; uint256 amountOutMinimum; uint160 sqrtPriceLimitX96; } function exactInputSingle(ExactInputSingleParams calldata params) external payable returns (uint256); }
contract StabilizationVault is Ownable, ReentrancyGuard {
    using SafeERC20 for IERC20;
    address public token0; address public token1; uint256 public reserve0; uint256 public reserve1;
    IUniswapV3Pool public pool; ISwapRouter public swapRouter; uint24 public fee; uint256 public rebalanceThresholdBps = 20; uint256 public targetPrice;
    bool public circuitBreakerActive = false; uint256 public lastExecutionTime; address public factory;
    event Deposit(address,uint256,uint256); event Withdraw(address,uint256,uint256); event ArbitrageExecuted(uint256,uint256,bool); event CircuitBreakerTriggered(string); event ParametersUpdated(string,uint256);
    constructor(address _token0, address _token1, uint24 _fee, address _factory, address _swapRouter, address _owner) Ownable(_owner) {
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
    function getPrice() public view returns (uint160 sqrtPriceX96, int24 tick) { (sqrtPriceX96, tick, , , , , ) = pool.slot0(); }
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
    function amountInToReachTarget(IUniswapV3Pool poolAddr, uint160 targetSqrtPriceX96) public view returns (uint256 amountInWithFee, int24 finalTick, uint128 finalLiquidity) {
        (uint160 sqrtPriceX96, int24 tick, , , , , ) = poolAddr.slot0();
        uint128 liquidity = poolAddr.liquidity();
        int24 tickSpacing = poolAddr.tickSpacing();
        require(liquidity > 0);
        require(targetSqrtPriceX96 != sqrtPriceX96);
        bool zeroForOne = sqrtPriceX96 > targetSqrtPriceX96;
        while (sqrtPriceX96 != targetSqrtPriceX96) {
            (int24 nextTick, ) = _nextInitializedTick(poolAddr, tick, tickSpacing, zeroForOne);
            if (nextTick < TickMath.MIN_TICK) nextTick = TickMath.MIN_TICK;
            if (nextTick > TickMath.MAX_TICK) nextTick = TickMath.MAX_TICK;
            uint160 sqrtPriceNextTickX96 = TickMath.getSqrtRatioAtTick(nextTick);
            uint160 stepTarget = zeroForOne ? (sqrtPriceNextTickX96 < targetSqrtPriceX96 ? targetSqrtPriceX96 : sqrtPriceNextTickX96) : (sqrtPriceNextTickX96 > targetSqrtPriceX96 ? targetSqrtPriceX96 : sqrtPriceNextTickX96);
            (uint256 amountIn, , uint160 sqrtPriceNextX96) = SwapMath.computeSwapStep(sqrtPriceX96, stepTarget, liquidity, type(int256).max, zeroForOne ? int256(fee) : 0, zeroForOne ? 0 : type(uint256).max);
            amountInWithFee += amountIn;
            sqrtPriceX96 = sqrtPriceNextX96;
            if (sqrtPriceX96 == sqrtPriceNextTickX96) {
                (, int128 liquidityNet, , , , , , ) = poolAddr.ticks(nextTick);
                liquidity = LiquidityMath.addDelta(liquidity, zeroForOne ? -liquidityNet : liquidityNet);
                tick = zeroForOne ? nextTick - 1 : nextTick;
            } else { tick = TickMath.getTickAtSqrtRatio(sqrtPriceX96); }
            if (sqrtPriceX96 == targetSqrtPriceX96) break;
        }
        finalTick = tick; finalLiquidity = liquidity;
    }
    function _nextInitializedTick(IUniswapV3Pool poolAddr, int24 tick, int24 ts, bool zf1) internal view returns (int24, bool) {
        int24 compressed = tick / ts;
        if (tick < 0 && tick % ts != 0) compressed--;
        return _nextInitializedTickWithinOneWord(poolAddr, compressed, ts, zf1);
    }
    function _nextInitializedTickWithinOneWord(IUniswapV3Pool poolAddr, int24 ct, int24 ts, bool lte) internal view returns (int24, bool) {
        (int16 wp, uint8 bp) = TickBitmap.position(ct);
        uint256 w = poolAddr.tickBitmap(wp);
        return TickBitmap.nextInitializedTickWithinOneWord(w, ct, ts, lte);
    }
    function _priceToSqrtPriceX96(uint256 p) internal pure returns (uint160) { return TickMath.getSqrtRatioAtTick(_priceToTick(p)); }
    function _priceToTick(uint256 p) internal pure returns (int24) { return TickMath.getTickAtSqrtRatio(TickMath.getSqrtRatioAtTick(p)); }
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
        uint256 amountOut = ISwapRouter(address(swapRouter)).exactInputSingle(ISwapRouter.ExactInputSingleParams({tokenIn: tokenIn, tokenOut: tokenOut, fee: fee, recipient: address(this), deadline: block.timestamp, amountIn: amountIn, amountOutMinimum: 0, sqrtPriceLimitX96: 0}));
        uint256 balInAfter = IERC20(tokenIn).balanceOf(address(this));
        uint256 balOutAfter = IERC20(tokenOut).balanceOf(address(this));
        require(amountOut > 0, "No output");
        if (zeroForOne) { reserve0 = balInAfter; reserve1 = balOutAfter; }
        else { reserve0 = balOutAfter; reserve1 = balInAfter; }
        lastExecutionTime = block.timestamp;
        emit ArbitrageExecuted(amountIn, amountOut, true);
        return true;
    }
    function triggerCircuitBreaker(string calldata reason) external onlyOwner { circuitBreakerActive = true; emit CircuitBreakerTriggered(reason); }
    function releaseCircuitBreaker() external onlyOwner { circuitBreakerActive = false; }
    function setRebalanceThresholdBps(uint256 _t) external onlyOwner { require(_t <= 1000); rebalanceThresholdBps = _t; emit ParametersUpdated("rebalanceThresholdBps", _t); }
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
    function emergencyWithdraw(address t, address to, uint256 a) external onlyOwner { require(to != address(0)); IERC20(t).safeTransfer(to, a); }
}
