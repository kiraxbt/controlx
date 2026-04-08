package chain

// Chain represents an EVM-compatible blockchain network.
type Chain struct {
	Name         string
	ChainID      int64
	Symbol       string // native token symbol
	RPCPath      string // path segment for Ankr RPC URL
	ExplorerURL  string
}

var (
	Ethereum = Chain{
		Name:        "Ethereum",
		ChainID:     1,
		Symbol:      "ETH",
		RPCPath:     "eth",
		ExplorerURL: "https://etherscan.io",
	}
	BSC = Chain{
		Name:        "BSC",
		ChainID:     56,
		Symbol:      "BNB",
		RPCPath:     "bsc",
		ExplorerURL: "https://bscscan.com",
	}
	Polygon = Chain{
		Name:        "Polygon",
		ChainID:     137,
		Symbol:      "MATIC",
		RPCPath:     "polygon",
		ExplorerURL: "https://polygonscan.com",
	}
	Arbitrum = Chain{
		Name:        "Arbitrum",
		ChainID:     42161,
		Symbol:      "ETH",
		RPCPath:     "arbitrum",
		ExplorerURL: "https://arbiscan.io",
	}
	Avalanche = Chain{
		Name:        "Avalanche",
		ChainID:     43114,
		Symbol:      "AVAX",
		RPCPath:     "avalanche",
		ExplorerURL: "https://snowtrace.io",
	}
	Optimism = Chain{
		Name:        "Optimism",
		ChainID:     10,
		Symbol:      "ETH",
		RPCPath:     "optimism",
		ExplorerURL: "https://optimistic.etherscan.io",
	}
	Base = Chain{
		Name:        "Base",
		ChainID:     8453,
		Symbol:      "ETH",
		RPCPath:     "base",
		ExplorerURL: "https://basescan.org",
	}
	Fantom = Chain{
		Name:        "Fantom",
		ChainID:     250,
		Symbol:      "FTM",
		RPCPath:     "fantom",
		ExplorerURL: "https://ftmscan.com",
	}
	ZkSync = Chain{
		Name:        "zkSync",
		ChainID:     324,
		Symbol:      "ETH",
		RPCPath:     "zksync_era",
		ExplorerURL: "https://explorer.zksync.io",
	}
	Scroll = Chain{
		Name:        "Scroll",
		ChainID:     534352,
		Symbol:      "ETH",
		RPCPath:     "scroll",
		ExplorerURL: "https://scrollscan.com",
	}
	Linea = Chain{
		Name:        "Linea",
		ChainID:     59144,
		Symbol:      "ETH",
		RPCPath:     "linea",
		ExplorerURL: "https://lineascan.build",
	}
	Mantle = Chain{
		Name:        "Mantle",
		ChainID:     5000,
		Symbol:      "MNT",
		RPCPath:     "mantle",
		ExplorerURL: "https://explorer.mantle.xyz",
	}
	Blast = Chain{
		Name:        "Blast",
		ChainID:     81457,
		Symbol:      "ETH",
		RPCPath:     "blast",
		ExplorerURL: "https://blastscan.io",
	}

	AllChains = []Chain{Ethereum, BSC, Polygon, Arbitrum, Avalanche, Optimism, Base, Fantom, ZkSync, Scroll, Linea, Mantle, Blast}
)

// Token represents a popular ERC-20 token on a specific chain.
type Token struct {
	Symbol  string
	Address string
}

// PopularTokens maps chain name to popular ERC-20 tokens.
var PopularTokens = map[string][]Token{
	"Ethereum": {
		{Symbol: "USDT", Address: "0xdAC17F958D2ee523a2206206994597C13D831ec7"},
		{Symbol: "USDC", Address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"},
		{Symbol: "DAI", Address: "0x6B175474E89094C44Da98b954EedeAC495271d0F"},
		{Symbol: "WETH", Address: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"},
		{Symbol: "WBTC", Address: "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599"},
	},
	"BSC": {
		{Symbol: "USDT", Address: "0x55d398326f99059fF775485246999027B3197955"},
		{Symbol: "USDC", Address: "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"},
		{Symbol: "DAI", Address: "0x1AF3F329e8BE154074D8769D1FFa4eE058B1DBc3"},
		{Symbol: "WBNB", Address: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"},
		{Symbol: "BUSD", Address: "0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56"},
	},
	"Polygon": {
		{Symbol: "USDT", Address: "0xc2132D05D31c914a87C6611C10748AEb04B58e8F"},
		{Symbol: "USDC", Address: "0x3c499c542cEF5E3811e1192ce70d8cC03d5c3359"},
		{Symbol: "DAI", Address: "0x8f3Cf7ad23Cd3CaDbD9735AFf958023239c6A063"},
		{Symbol: "WMATIC", Address: "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270"},
		{Symbol: "WETH", Address: "0x7ceB23fD6bC0adD59E62ac25578270cFf1b9f619"},
	},
	"Arbitrum": {
		{Symbol: "USDT", Address: "0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9"},
		{Symbol: "USDC", Address: "0xaf88d065e77c8cC2239327C5EDb3A432268e5831"},
		{Symbol: "DAI", Address: "0xDA10009cBd5D07dd0CeCc66161FC93D7c9000da1"},
		{Symbol: "WETH", Address: "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1"},
		{Symbol: "WBTC", Address: "0x2f2a2543B76A4166549F7aaB2e75Bef0aefC5B0f"},
	},
	"Avalanche": {
		{Symbol: "USDT", Address: "0x9702230A8Ea53601f5cD2dc00fDBc13d4dF4A8c7"},
		{Symbol: "USDC", Address: "0xB97EF9Ef8734C71904D8002F8b6Bc66Dd9c48a6E"},
		{Symbol: "DAI.e", Address: "0xd586E7F844cEa2F87f50152665BCbc2C279D8d70"},
		{Symbol: "WAVAX", Address: "0xB31f66AA3C1e785363F0875A1B74E27b85FD66c7"},
		{Symbol: "WETH.e", Address: "0x49D5c2BdFfac6CE2BFdB6640F4F80f226bc10bAB"},
	},
	"Optimism": {
		{Symbol: "USDT", Address: "0x94b008aA00579c1307B0EF2c499aD98a8ce58e58"},
		{Symbol: "USDC", Address: "0x0b2C639c533813f4Aa9D7837CAf62653d097Ff85"},
		{Symbol: "DAI", Address: "0xDA10009cBd5D07dd0CeCc66161FC93D7c9000da1"},
		{Symbol: "WETH", Address: "0x4200000000000000000000000000000000000006"},
	},
	"Base": {
		{Symbol: "USDC", Address: "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"},
		{Symbol: "DAI", Address: "0x50c5725949A6F0c72E6C4a641F24049A917DB0Cb"},
		{Symbol: "WETH", Address: "0x4200000000000000000000000000000000000006"},
		{Symbol: "cbETH", Address: "0x2Ae3F1Ec7F1F5012CFEab0185bfc7aa3cf0DEc22"},
	},
	"Fantom": {
		{Symbol: "fUSDT", Address: "0x049d68029688eAbF473097a2fC38ef61633A3C7A"},
		{Symbol: "USDC", Address: "0x04068DA6C83AFCFA0e13ba15A6696662335D5B75"},
		{Symbol: "DAI", Address: "0x8D11eC38a3EB5E956B052f67Da8Bdc9bef8Abf3E"},
		{Symbol: "WFTM", Address: "0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83"},
	},
	"zkSync": {
		{Symbol: "USDC", Address: "0x1d17CBcF0D6D143135aE902365D2E5e2A16538D4"},
		{Symbol: "USDT", Address: "0x493257fD37EDB34451f62EDf8D2a0C418852bA4C"},
		{Symbol: "WETH", Address: "0x5AEa5775959fBC2557Cc8789bC1bf90A239D9a91"},
		{Symbol: "WBTC", Address: "0xBBeB516fb02a01611cBBE0453Fe3c580D7281011"},
	},
	"Scroll": {
		{Symbol: "USDC", Address: "0x06eFdBFf2a14a7c8E15944D1F4A48F9F95F663A4"},
		{Symbol: "USDT", Address: "0xf55BEC9cafDbE8730f096Aa55dad6D22d44099Df"},
		{Symbol: "WETH", Address: "0x5300000000000000000000000000000000000004"},
		{Symbol: "DAI", Address: "0xcA77eB3fEFe3725Dc33bceB7942C08ad994D5cEC"},
	},
	"Linea": {
		{Symbol: "USDC", Address: "0x176211869cA2b568f2A7D4EE941E073a821EE1ff"},
		{Symbol: "USDT", Address: "0xA219439258ca9da29E9Cc4cE5596924745e12B93"},
		{Symbol: "WETH", Address: "0xe5D7C2a44FfDDf6b295A15c148167daaAf5Cf34f"},
		{Symbol: "DAI", Address: "0x4AF15ec2A0BD43Db75dd04E62FAA3B8EF36b00d5"},
	},
	"Mantle": {
		{Symbol: "USDC", Address: "0x09Bc4E0D10E52d89AbeAc913530aE5f2309120EA"},
		{Symbol: "USDT", Address: "0x201EBa5CC46D216Ce6DC03F6a759669559675521"},
		{Symbol: "WETH", Address: "0xdEAddEaDdeadDEadDEADDEAddEADDEAddead1111"},
		{Symbol: "WMNT", Address: "0x78c1b0C915c4FAA5FffA6CAbf0219DA63d7f4cb8"},
	},
	"Blast": {
		{Symbol: "USDB", Address: "0x4300000000000000000000000000000000000003"},
		{Symbol: "WETH", Address: "0x4300000000000000000000000000000000000004"},
	},
}
