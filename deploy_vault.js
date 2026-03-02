const fs = require('fs');
const path = require('path');
const solc = require('solc');
const { ethers } = require('ethers');

const RPC_URL = "https://astrochain-sepolia.gateway.tenderly.co/5neqYQoinBsj3Cc3O36Dun";
const PRIVATE_KEY = "0x298149d01f7a23cb938ab6874ea345516479fb70bd5e14c99c0ffaf84798ca80";

const TOKEN0 = "0x2d7efff683b0a21e0989729e0249c42cdf9ee442";
const TOKEN1 = "0x948e15b38f096d3a664fdeef44c13709732b2110";
const FEE = 100;
const FACTORY = "0x1F98431c8aD98523631AE4a59f267346ea31F984";
const SWAP_ROUTER = "0xd1AAE39293221B77B0C71fBD6dCb7Ea29Bb5B166";

const source = fs.readFileSync(path.join(__dirname, 'contracts', 'StabilizationVault.sol'), 'utf8');

const input = {
    language: 'Solidity',
    sources: { 'StabilizationVault.sol': { content: source } },
    settings: { outputSelection: { '*': { '*': ['*'] } } }
};

const output = solc.compile(JSON.stringify(input));
const compiled = JSON.parse(output);

if (compiled.errors) {
    compiled.errors.forEach(e => {
        if (e.severity === 'error') console.log('ERROR:', e.message);
    });
}

const bytecode = compiled.contracts['StabilizationVault.sol'].StabilizationVault.evm.bytecode.object;
const abi = compiled.contracts['StabilizationVault.sol'].StabilizationVault.abi;

console.log('Compiled! Bytecode length:', bytecode.length);

async function deploy() {
    const provider = new ethers.JsonRpcProvider(RPC_URL);
    const wallet = new ethers.Wallet(PRIVATE_KEY, provider);
    
    console.log('Deploying from:', wallet.address);
    
    const factory = new ethers.ContractFactory(abi, bytecode, wallet);
    const contract = await factory.deploy(
        TOKEN0, TOKEN1, FEE, FACTORY, SWAP_ROUTER, wallet.address
    );
    
    console.log('Tx:', contract.deploymentTransaction().hash);
    await contract.waitForDeployment();
    console.log('Deployed to:', await contract.getAddress());
}

deploy().catch(e => console.error('Deploy failed:', e.message));
