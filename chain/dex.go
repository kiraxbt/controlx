package chain

// DexRouter represents a Uniswap V2-compatible DEX router.
type DexRouter struct {
	Address string
	Name    string
}

// DexRouters maps chain name to its primary Uniswap V2-fork DEX router.
var DexRouters = map[string]DexRouter{
	"Ethereum":  {Address: "0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D", Name: "Uniswap V2"},
	"BSC":       {Address: "0x10ED43C718714eb63d5aA57B78B54704E256024E", Name: "PancakeSwap"},
	"Polygon":   {Address: "0xa5E0829CaCEd8fFDD4De3c43696c57F7D7A678ff", Name: "QuickSwap"},
	"Arbitrum":  {Address: "0x1b02dA8Cb0d097eB8D57A175b88c7D8b47997506", Name: "SushiSwap"},
	"Avalanche": {Address: "0x60aE616a2155Ee3d9A68541Ba4544862310933d4", Name: "TraderJoe"},
	"Optimism":  {Address: "0x4A7b5Da61326A6379179b40d00F57E5bbDC962c2", Name: "Velodrome"},
	"Base":      {Address: "0x4752ba5DBc23f44D87826276BF6Fd6b1C372aD24", Name: "Aerodrome"},
	"Fantom":    {Address: "0xF491e7B69E4244ad4002BC14e878a34207E38c29", Name: "SpookySwap"},
	"zkSync":    {Address: "0x2da10A1e27bF85cEdD8FFb1AbBe97e53391C0295", Name: "SyncSwap"},
	"Scroll":    {Address: "0x80e38291e06339d10AAB483C65695D004dBD5C69", Name: "Ambient"},
	"Linea":     {Address: "0xF5bB79d1E97c8D97acf8b268C1F57D2C1a26aB03", Name: "Lynex"},
	"Mantle":    {Address: "0x319B69888b0d11cEC22caA5034e25FfFBDc88421", Name: "FusionX"},
	"Blast":     {Address: "0x44889b52b71E60De6ed7dE82E2939fcc52fB2B4E", Name: "Thruster"},
}

// WrappedNative maps chain name to wrapped native token address.
var WrappedNative = map[string]string{
	"Ethereum":  "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
	"BSC":       "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c",
	"Polygon":   "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270",
	"Arbitrum":  "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1",
	"Avalanche": "0xB31f66AA3C1e785363F0875A1B74E27b85FD66c7",
	"Optimism":  "0x4200000000000000000000000000000000000006",
	"Base":      "0x4200000000000000000000000000000000000006",
	"Fantom":    "0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83",
	"zkSync":    "0x5AEa5775959fBC2557Cc8789bC1bf90A239D9a91",
	"Scroll":    "0x5300000000000000000000000000000000000004",
	"Linea":     "0xe5D7C2a44FfDDf6b295A15c148167daaAf5Cf34f",
	"Mantle":    "0x78c1b0C915c4FAA5FffA6CAbf0219DA63d7f4cb8",
	"Blast":     "0x4300000000000000000000000000000000000004",
}

// CoinGeckoIDs maps chain name to CoinGecko platform ID for price queries.
var CoinGeckoIDs = map[string]string{
	"Ethereum":  "ethereum",
	"BSC":       "binancecoin",
	"Polygon":   "matic-network",
	"Arbitrum":  "ethereum",
	"Avalanche": "avalanche-2",
	"Optimism":  "ethereum",
	"Base":      "ethereum",
	"Fantom":    "fantom",
	"zkSync":    "ethereum",
	"Scroll":    "ethereum",
	"Linea":     "ethereum",
	"Mantle":    "mantle",
	"Blast":     "ethereum",
}
