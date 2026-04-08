<p align="center">
  <img src="app.png" alt="CONTROLX" width="700">
</p>

```
    ██████╗ ██████╗ ███╗   ██╗████████╗██████╗  ██████╗ ██╗     ██╗  ██╗
   ██╔════╝██╔═══██╗████╗  ██║╚══██╔══╝██╔══██╗██╔═══██╗██║     ╚██╗██╔╝
   ██║     ██║   ██║██╔██╗ ██║   ██║   ██████╔╝██║   ██║██║      ╚███╔╝
   ██║     ██║   ██║██║╚██╗██║   ██║   ██╔══██╗██║   ██║██║      ██╔██╗
   ╚██████╗╚██████╔╝██║ ╚████║   ██║   ██║  ██║╚██████╔╝███████╗██╔╝ ██╗
    ╚═════╝ ╚═════╝ ╚═╝  ╚═══╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝╚══════╝╚═╝  ╚═╝
```

# CONTROLX

**EVM Multi-Wallet Manager** — Terminal-based tool for managing hundreds of EVM wallets across 13 chains. Generate, distribute, mix, bridge, swap, and track — all from one TUI.

---

## Table of Contents

- [Features](#features)
- [Supported Chains](#supported-chains)
- [Installation](#installation)
- [Configuration](#configuration)
- [Quick Start](#quick-start)
- [Usage Guide](#usage-guide)
- [Bridge Mix Tutorial](#bridge-mix-tutorial)
- [DEX Mix Tutorial](#dex-mix-tutorial)
- [Alerts Setup](#alerts-setup)
- [Proxy Setup](#proxy-setup)
- [Keyboard Shortcuts](#keyboard-shortcuts)
- [File Reference](#file-reference)
- [Troubleshooting](#troubleshooting)
- [Security](#security)
- [Disclaimer](#disclaimer)

---

## Features

| Category | Features |
|----------|----------|
| **Wallet** | Generate batch wallets, AES-256-GCM encrypted storage, group system, labels/tags, encrypted backup & restore |
| **Operations** | Check balances (native + ERC-20), distribute (1 to many), collect (many to 1), sweep all, auto-fund gas |
| **Mix / Privacy** | Cross-chain bridge mix (Li.Fi FASTEST), DEX swap mix, amount randomization, gaussian delay |
| **Analytics** | Portfolio dashboard (USD via CoinGecko), gas cost estimator, dry-run simulation |
| **Settings** | Telegram/Discord alerts, SOCKS5/HTTP proxy rotation, session auto-lock, operation queue |

---

## Supported Chains

| # | Chain | Symbol | DEX Router |
|---|-------|--------|------------|
| 1 | Ethereum | ETH | Uniswap V2 |
| 2 | BSC | BNB | PancakeSwap |
| 3 | Polygon | MATIC | QuickSwap |
| 4 | Arbitrum | ETH | SushiSwap |
| 5 | Avalanche | AVAX | TraderJoe |
| 6 | Optimism | ETH | Velodrome |
| 7 | Base | ETH | Aerodrome |
| 8 | Fantom | FTM | SpookySwap |
| 9 | zkSync Era | ETH | SyncSwap |
| 10 | Scroll | ETH | Ambient |
| 11 | Linea | ETH | Lynex |
| 12 | Mantle | MNT | FusionX |
| 13 | Blast | ETH | Thruster |

Each chain includes popular ERC-20 tokens (USDT, USDC, DAI, WETH, WBTC, etc.).

---

## Installation

### Prerequisites

- **Go 1.21+**
- **Ankr RPC key** — free at [ankr.com/rpc](https://www.ankr.com/rpc/)

### Setup

```bash
# 1. Initialize module
go mod init controlx
go mod tidy

# 2. Build
go build -o controlx .
```

### Verify

```bash
./controlx
```

If you see the CONTROLX banner and menu, you're ready.

---

## Configuration

### RPC Keys (Required)

Create `ankr.txt` in the project directory with your Ankr RPC key(s), one per line:

```
your_first_ankr_key_here
your_second_ankr_key_here
```

Multiple keys enable **round-robin load balancing** and **automatic failover** — if one key hits rate limits, it rotates to the next.

> **WARNING:** `ankr.txt` contains your private API keys. It is listed in `.gitignore` and will NOT be committed. Never share this file.

### Proxy List (Optional)

Create `proxies.txt` for IP rotation:

```
socks5://host:port
socks5://user:pass@host:port
http://host:port
http://user:pass@host:port
```

---

## Quick Start

```bash
# 1. Add your RPC key
echo "YOUR_ANKR_KEY_HERE" > ankr.txt

# 2. Run
./controlx
```

Main menu:

```
   wallets: ---    delay: OFF    rpc: 1 keys
   proxy: OFF    alert: OFF

   WALLET
    > Generate new wallets
      Load existing wallets
      Wallet labels
      Backup / Restore
   OPERATIONS
      Check all balances
      Distribute (1 -> many)
      Collect (many -> 1)
      Sweep all funds
      Auto-fund gas
   MIX / PRIVACY
      Mix (cross-chain bridge)
      DEX Mix (swap)
   ANALYTICS
      Portfolio dashboard
      Gas tracker
      Dry-run simulate
   SETTINGS
      Humanizer delay
      Alerts (Telegram/Discord)
      Proxy settings
      Session timeout
      Operation queue
      Export addresses (CSV)
      Transaction log
```

---

## Usage Guide

### 1. Wallet Management

#### Generate Wallets

`Generate new wallets` > enter group name > enter count > set password > confirm password

```
> Generated 500 wallets in 1.823s
> Saved to wallets_group-001.json (encrypted)
```

#### Load Wallets

`Load existing wallets` > select group > enter password

Wallets remain in memory for the session. You must load before using any operation.

#### Wallet Labels

`Wallet labels` > `Set label` > enter wallet index > enter label (e.g., `CEX`, `hot`, `cold`)

View all labeled wallets via `View labels`. Labels persist across sessions.

#### Backup / Restore

**Create:** `Backup / Restore` > `Create backup` > filename > password

Creates a single AES-256-GCM encrypted file with all wallet groups + labels.

**Restore:** `Restore backup` > filename > password

Restores all wallet files and group index.

---

### 2. Operations

#### Check Balances

Select chain > select token (native or ERC-20) > view results

Shows per-wallet balance with totals. Non-zero wallets highlighted.

#### Distribute (1 to many)

Select chain > source wallet index > destination range (`2-100` or `all`) > select token > amount per wallet > confirm

Sends the same amount from one wallet to many. Sequential nonce for reliability.

#### Collect (many to 1)

Select chain > destination address > select token > confirm

Consolidates funds from all loaded wallets to one address. Runs concurrently (50 goroutines).

#### Sweep All Funds

Select chain > destination address > select token > confirm

Sends ALL available balance (minus gas) from every wallet. Skips zero-balance wallets automatically.

#### Auto-fund Gas

Select chain > funder wallet index > minimum gas amount > review scan > confirm

Scans all wallets, identifies those below the gas threshold, and funds them from the source wallet.

---

### 3. Mix / Privacy

#### Bridge Mix (Cross-chain)

Route funds through wallets across different chains via Li.Fi bridge.

1. Enter chain rotation (e.g., `3,1,2,5,9` = Polygon > Ethereum > BSC > Avalanche > zkSync)
2. Enter source wallet index
3. Enter number of hops
4. Confirm and watch live progress

**Key features:**
- Li.Fi `order=FASTEST` — picks the quickest bridge, not the cheapest
- Adaptive gas retry — reduces amount 5% and re-quotes on insufficient funds (up to 6 retries)
- Smart timeout — uses bridge ETA from Li.Fi (min 2min, max 10min)
- Detailed bridge tree output with wallet numbers and balances

See [Bridge Mix Tutorial](#bridge-mix-tutorial) below.

#### DEX Mix (Swap)

Route funds through on-chain swaps across wallets.

1. Select chain
2. Select token (e.g., USDC)
3. Enter source wallet index
4. Enter number of hops
5. Enter slippage (default: 100 bps = 1%)
6. Confirm

See [DEX Mix Tutorial](#dex-mix-tutorial) below.

---

### 4. Analytics

#### Portfolio Dashboard

Scans native balance across all 13 chains with USD pricing via CoinGecko.

```
CHAIN         BALANCE     WALLETS       PRICE     USD VALUE
-------------------------------------------------------------------
Ethereum      0.5000 ETH        5     $3,200.00     $1,600.00
BSC           2.3400 BNB        8       $580.00     $1,357.20
Polygon     450.0000 MATIC     12         $0.72       $324.00
-------------------------------------------------------------------

TOTAL PORTFOLIO:  $3,281.20 USD
```

#### Gas Tracker

Estimate gas cost for batch operations before executing.

Select chain > number of transactions > gas limit per tx

Shows total gas in native token + USD equivalent.

#### Dry-Run Simulate

Simulate distribute, sweep, or dexmix without broadcasting. Reports gas cost, source balance, and whether the operation is affordable.

---

### 5. Settings

#### Humanizer Delay

Choose from 6 presets (uniform or gaussian distribution) or set custom range.

| Preset | Range | Mode |
|--------|-------|------|
| Conservative | 5-15s | Uniform / Gaussian |
| Moderate | 3-10s | Uniform / Gaussian |
| Aggressive | 1-5s | Uniform / Gaussian |
| Custom | min-max | - |

**Gaussian mode:** delays cluster around the center — looks more natural.

**Amount randomizer:** adds random variance to tx amounts (e.g., `10` = each amount varies by +/-10%).

#### Session Timeout

Set auto-lock after N minutes of inactivity. The screen locks and requires Enter to resume.

#### Operation Queue

Chain multiple operations with delay steps between them. Add steps, view queue, execute sequentially.

---

## Bridge Mix Tutorial

### Scenario

Fund in wallet #1 on Ethereum, route through 6 wallets across 3 chains.

### Steps

1. Load wallets, set delay (recommended: gaussian)
2. `Mix (cross-chain bridge)`
3. Chain rotation: `3,1,2,5,9` (Polygon > Ethereum > BSC > Avalanche > zkSync)

```
Chain rotation  e.g. 3,1,2,5,9
    1 = Ethereum     (ETH)
    2 = BSC          (BNB)
    3 = Polygon      (MATIC)
    4 = Arbitrum     (ETH)
    5 = Avalanche    (AVAX)
    6 = Optimism     (ETH)
    7 = Base         (ETH)
    8 = Fantom       (FTM)
    9 = zkSync       (ETH)
   10 = Scroll       (ETH)
   11 = Linea        (ETH)
   12 = Mantle       (MNT)
   13 = Blast        (ETH)
```

4. Source wallet: `1`, hops: `6`
5. Confirm

### Live Progress

```
  == BridgeMix START ==
  rotation=Ethereum > Polygon > BSC wallets=7 hops=6 wallet#1-#7
  hop 0: wallet#1 on Ethereum > wallet#2 on Polygon
  hop 0: wallet#1 balance=0.0500 ETH on Ethereum
  hop 0: bridging 0.0498 ETH via Li.Fi (Ethereum > Polygon)...
  hop 0: bridge=Stargate estOut=120.45 ETA=~30s
  hop 0: tx sent 0x982a..., waiting for confirmation...
  hop 0 OK: wallet#1>#2 Ethereum>Polygon via Stargate amount=0.0498
  hop 0: waiting for bridge arrival on Polygon (wallet#2) (bridge ETA ~30s, timeout 1m30s)
  waiting: bridge confirmed! 0.0000 > 120.4500 on Polygon
```

### Result Tree

```
  BRIDGE MIX -- CROSS-CHAIN TRANSFER TREE

  rotation: Ethereum > Polygon > BSC

  wallet #1  0x1a2B...3c4D
  | Ethereum
  |
  |-- HOP 0  Ethereum > Polygon
  |   bridge  : Stargate  (~30s)
  |   sent    : 0.0498 Ethereum
  |   received: 120.4500 Polygon
  |   tx      : 0x982a8d53c511edeb...
  |
  |  wallet #2  0x5e6F...7g8H
  |     Polygon  balance: 120.4500
  |
  |-- HOP 1  Polygon > BSC
  |   bridge  : Stargate  (~45s)
  |   sent    : 120.1200 Polygon
  |   received: 0.4100 BSC
  |   ...
  ...

  FINAL DESTINATION
  wallet #7  0x9f3a...b2e1
     chain   : Ethereum
     balance : 0.0420
     status  : CLEAN WALLET
```

### How Adaptive Retry Works

If a bridge tx fails with "insufficient funds":

```
Attempt 0: amount=252.84 MATIC > "insufficient funds"
Attempt 1: amount=240.20 MATIC (-5%) > re-quote from Li.Fi > "insufficient funds"
Attempt 2: amount=228.19 MATIC (-5%) > re-quote from Li.Fi > SUCCESS
```

Each retry: wait > reduce 5% > re-check balance > re-fetch gas price > re-quote > verify > send.

---

## DEX Mix Tutorial

### Scenario

Swap ETH through USDC across 4 wallets on Arbitrum.

### Steps

1. Load wallets, set delay
2. `DEX Mix (swap)` > Arbitrum > USDC > source #1 > 3 hops > slippage 100 > confirm

### Flow

```
Wallet #1: 0.05 ETH
  | swapExactETHForTokens > USDC to Wallet #2
  v
Wallet #2: 120 USDC
  | approve + swapExactTokensForETH > ETH to Wallet #3
  v
Wallet #3: 0.048 ETH
  | swapExactETHForTokens > USDC to Wallet #4
  v
Wallet #4: 118 USDC
```

**Important:** Odd-index wallets (receiving tokens) need native gas for the approve + swap transactions. Run **Auto-fund gas** first.

---

## Alerts Setup

### Telegram

1. Message [@BotFather](https://t.me/BotFather) > `/newbot` > get your **Bot Token**
2. Send any message to your bot, then visit `https://api.telegram.org/bot<TOKEN>/getUpdates` to get your **Chat ID**
3. In app: `Alerts` > `Telegram` > enter bot token > enter chat ID

### Discord

1. Server Settings > Integrations > Webhooks > New Webhook > Copy URL
2. In app: `Alerts` > `Discord` > paste webhook URL

Alerts fire automatically after Bridge Mix, DEX Mix, Distribute, Collect, and Sweep operations.

---

## Proxy Setup

Create `proxies.txt`:

```
socks5://127.0.0.1:1080
socks5://user:pass@proxy.example.com:1080
http://proxy.example.com:8080
```

In app: `Proxy settings` > `Load proxy file` > enter filename

Toggle between **Round-Robin** (sequential) and **Random** rotation modes.

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Up/Down` or `j/k` | Navigate |
| `Enter` | Select / Confirm |
| `Esc` | Cancel / Back |
| `y` / `n` | Quick Yes / No |
| `Left/Right` or `h/l` | Toggle confirm buttons |
| `q` | Quit (main menu) |
| `Ctrl+C` | Force quit |

**In scrollable results:**

| Key | Action |
|-----|--------|
| `Up/Down` or `j/k` | Scroll |
| `Enter` / `Esc` | Back to menu |

---

## File Reference

| File | Purpose | In `.gitignore` |
|------|---------|:---:|
| `ankr.txt` | RPC API keys | Yes |
| `wallets_*.json` | Encrypted wallet files | Yes |
| `wallet_groups.json` | Wallet group index | Yes |
| `wallet_labels.json` | Wallet labels/tags | Yes |
| `presets.json` | Saved operation presets | Yes |
| `proxies.txt` | Proxy list | Yes |
| `tx_log.csv` | Transaction log (CSV) | Yes |
| `tx_log.json` | Transaction log (JSON) | Yes |
| `bridge.log` | Bridge debug log | Yes |
| `*.enc` | Encrypted backup files | Yes |
| `go.mod` | Go module definition | No |
| `go.sum` | Dependency checksums | No |
| `main.go` | Application entry point | No |
| `chain/` | Chain definitions, RPC provider | No |
| `ops/` | All operation logic | No |
| `wallet/` | Wallet generation, storage, backup | No |

---

## Troubleshooting

### "Failed to load RPC keys"

`ankr.txt` is missing or empty. Create it with at least one valid Ankr RPC key.

### "No wallets loaded"

You must **Generate** or **Load** wallets before running operations. Wallets are not persisted in memory between sessions.

### "insufficient funds for gas"

Bridge Mix automatically retries up to 6 times, reducing amount by 5% each attempt. If it still fails, the wallet balance is too low for the bridge contract. Use a wallet with more funds or try a different chain rotation.

### Bridge arrival takes too long

The app uses `order=FASTEST` on Li.Fi to select the quickest bridge. Timeout is set to 3x the estimated bridge time. Some chain pairs are inherently slower.

### "decrypt (wrong password?)"

Wallet encryption is AES-256-GCM with Scrypt KDF. Passwords are case-sensitive. There is no recovery if the password is lost.

### DEX swap reverted

- Increase slippage to 200-500 bps
- Check token liquidity on the DEX
- Some tokens have transfer taxes that cause reverts

### CoinGecko errors

The free API has rate limits. Prices are cached for 5 minutes. Wait and retry.

---

## Security

- **Encryption:** AES-256-GCM with Scrypt key derivation (N=32768, r=8, p=1)
- **No telemetry:** Only connects to Ankr RPC, Li.Fi API, and CoinGecko
- **Local storage:** All data stays on your machine
- **Password handling:** Never stored on disk — derived into key and discarded
- **File permissions:** Wallet files created with mode 0600
- **Session lock:** Auto-lock after configurable idle timeout

---

## Disclaimer

**THIS IS NOT FINANCIAL ADVICE.**

This software is provided as-is, free and open source, for educational and research purposes. The creators and contributors:

- Are **NOT** responsible for any financial losses arising from the use of this software
- Are **NOT** responsible for any illegal activity conducted using this software
- Do **NOT** provide financial, legal, or investment advice of any kind
- Make **NO** guarantees about the accuracy, reliability, or suitability of this software

If this software is used for any illegal, fraudulent, or otherwise unlawful purpose, **that is entirely the responsibility of the user** and falls outside the scope and intent of this project. The creators bear no liability whatsoever.

Always test with small amounts on testnets first. **Use at your own risk.**

By using this software, you acknowledge that you are solely responsible for your own actions and any consequences that may result.

---

## License

MIT License — Free for all. See [LICENSE](LICENSE) for full text.

This software is **100% open source** and **free to use, modify, and distribute** by anyone for any purpose under the terms of the MIT License.

---

## Donations

If this tool saved you time, consider buying me a coffee.

| Network | Address |
|---------|---------|
| **XMR** | `49284mUPqYmbm9nLNjGPuKgS56nhRtawGfxJXa6TK6dF5aVD8UdiGX3KvDwrBdK5y25kK8ieh6J8x28uXy3RjjEwJecRbwF` |
| **ETH / EVM** | `0x3F5759c94926dDe12D9C7444177e0AFdc30c0587` |

No obligations. The code is free forever.
