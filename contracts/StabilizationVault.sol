// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

interface IERC20 { 
    function transfer(address to, uint256 amount) external returns (bool); 
    function transferFrom(address from, address to, uint256 amount) external returns (bool); 
    function balanceOf(address account) external view returns (uint256); 
    function approve(address spender, uint256 amount) external returns (bool); 
} 

library SafeERC20 {
    function safeTransfer(IERC20 t, address to, uint256 a) internal { (bool s,) = address(t).call(abi.encodeCall(IERC20.transfer, (to, a))); require(s); }
    function safeTransferFrom(IERC20 t, address f, address to, uint256 a) internal { (bool s,) = address(t).call(abi.encodeCall(IERC20.transferFrom, (f, to, a))); require(s); }
    function forceApprove(IERC20 t, address s, uint256 v) internal { (bool s,) = address(t).call(abi.encodeCall(IERC20.approve, (s, v))); require(s); }
}

interface IUniswapV3Pool { 
    function slot0() external view returns (uint160 sqrtPriceX96, int24 tick, uint16 observationCardinality, uint16 observationCardinalityNext, uint16 feeProtocol, uint8 unlocked, bool);
    function liquidity() external view returns (uint128);
} 

interface IUniswapV3Factory { function getPool(address, address, uint24) external view returns (address); }

interface ISwapRouter { 
    struct ExactInputSingleParams { 
        address tokenIn; 
        address tokenOut; 
        uint24 fee; 
        address recipient; 
        uint256 deadline; 
        uint256 amountIn; 
        uint256 amountOutMinimum; 
        uint160 sqrtPriceLimitX96; 
    } 
    function exactInputSingle(ExactInputSingleParams calldata params) external payable returns (uint256 amountOut);
}

contract StabilizationVault is Ownable, ReentrancyGuard {
    using SafeERC20 for IERC20;
    
    address public token0;
    address public token1;
    uint256 public reserve0;
    uint256 public reserve1;
    IUniswapV3Pool public pool;
    ISwapRouter public swapRouter;
    uint24 public fee;
    uint256 public rebalanceThresholdBps = 20;
    uint256 public targetPrice = 1;
    bool public circuitBreakerActive = false;
    address public factory;

    event Deposit(address indexed user, uint256 amount0, uint256 amount1);
    event Withdraw(address indexed user, uint256 amount0, uint256 amount1);
    event ArbitrageExecuted(uint256 amountIn, uint256 amountOut, bool success);
    event CircuitBreakerTriggered(string reason);
    event ParametersUpdated(string name, uint256 value);

    constructor(
        address _token0, 
        address _token1, 
        uint24 _fee, 
        address _factory, 
        address _swapRouter, 
        address _owner
    ) Ownable(_owner) {
        token0 = _token0;
        token1 = _token1;
        factory = _factory;
        pool = IUniswapV3Pool(IUniswapV3Factory(_factory).getPool(token0, token1, _fee));
        fee = _fee;
        swapRouter = ISwapRouter(_swapRouter);
        targetPrice = 1;
    }

    function deposit(uint256 amount0, uint256 amount1) external onlyOwner nonReentrant {
        require(amount0 > 0 || amount1 > 0, "Zero amount");
        if (amount0 > 0) IERC20(token0).safeTransferFrom(msg.sender, address(this), amount0);
        if (amount1 > 0) IERC20(token1).safeTransferFrom(msg.sender, address(this), amount1);
        reserve0 += amount0;
        reserve1 += amount1;
        emit Deposit(msg.sender, amount0, amount1);
    }

    function withdraw(uint256 amount0, uint256 amount1) external onlyOwner nonReentrant {
        require(amount0 > 0 || amount1 > 0, "Zero amount");
        require(amount0 <= reserve0 && amount1 <= reserve1, "Insufficient reserves");
        reserve0 -= amount0;
        reserve1 -= amount1;
        if (amount0 > 0) IERC20(token0).safeTransfer(msg.sender, amount0);
        if (amount1 > 0) IERC20(token1).safeTransfer(msg.sender, amount1);
        emit Withdraw(msg.sender, amount0, amount1);
    }

    function getPrice() public view returns (uint160 sqrtPriceX96, int24 tick) {
        (sqrtPriceX96, tick, , , , , ) = pool.slot0();
    }

    function getLiquidity() public view returns (uint128) {
        return pool.liquidity();
    }

    /// @notice Calculate swap amount based on price difference
    function calculateSwapAmount(uint256) public view returns (uint256 amountIn) {
        (uint160 sqrtPriceX96, , , , , , ) = pool.slot0();
        uint128 liquidity = pool.liquidity();
        if (liquidity == 0) return 0;
        
        // Use 1% of liquidity as amount
        amountIn = uint256(liquidity) / 100;
    }

    function executeArbitrage() external onlyOwner nonReentrant returns (bool) {
        require(!circuitBreakerActive, "Circuit breaker active");
        
        (uint160 sqrtPriceX96, ) = getPrice();
        uint256 currentPrice = (uint256(sqrtPriceX96) ** 2) >> 192;
        
        uint256 priceDiff = currentPrice > targetPrice ? currentPrice - targetPrice : targetPrice - currentPrice;
        uint256 deviationBps = (priceDiff * 10000) / targetPrice;
        require(deviationBps >= rebalanceThresholdBps, "Price deviation below threshold");
        
        bool zeroForOne = currentPrice > targetPrice;
        address tokenIn = zeroForOne ? token0 : token1;
        address tokenOut = zeroForOne ? token1 : token0;
        
        uint256 amountIn = calculateSwapAmount(targetPrice);
        require(amountIn > 0, "Invalid amount");
        
        uint256 balBefore = IERC20(tokenIn).balanceOf(address(this));
        require(balBefore >= amountIn, "Insufficient balance");
        
        IERC20(tokenIn).forceApprove(address(swapRouter), amountIn);
        
        uint256 amountOut = ISwapRouter(address(swapRouter)).exactInputSingle(ISwapRouter.ExactInputSingleParams({
            tokenIn: tokenIn,
            tokenOut: tokenOut,
            fee: fee,
            recipient: address(this),
            deadline: block.timestamp,
            amountIn: amountIn,
            amountOutMinimum: 0,
            sqrtPriceLimitX96: 0
        }));
        
        require(amountOut > 0, "No output");
        
        uint256 balanceInAfter = IERC20(tokenIn).balanceOf(address(this));
        uint256 balanceOutAfter = IERC20(tokenOut).balanceOf(address(this));
        
        if (zeroForOne) {
            reserve0 = balanceInAfter;
            reserve1 = balanceOutAfter;
        } else {
            reserve0 = balanceOutAfter;
            reserve1 = balanceInAfter;
        }
        
        emit ArbitrageExecuted(amountIn, amountOut, true);
        return true;
    }

    function triggerCircuitBreaker(string calldata reason) external onlyOwner {
        circuitBreakerActive = true;
        emit CircuitBreakerTriggered(reason);
    }

    function releaseCircuitBreaker() external onlyOwner {
        circuitBreakerActive = false;
    }

    function setRebalanceThresholdBps(uint256 _t) external onlyOwner {
        require(_t <= 1000);
        rebalanceThresholdBps = _t;
        emit ParametersUpdated("rebalanceThresholdBps", _t);
    }

    function setTargetPrice(uint256 _p) external onlyOwner {
        targetPrice = _p;
        emit ParametersUpdated("targetPrice", _p);
    }

    function getReserves() external view returns (uint256, uint256) {
        return (reserve0, reserve1);
    }

    function getVaultTVL() external view returns (uint256) {
        return reserve0 + reserve1;
    }

    function emergencyWithdraw(address t, address to, uint256 a) external onlyOwner {
        require(to != address(0));
        IERC20(t).safeTransfer(to, a);
    }
}
