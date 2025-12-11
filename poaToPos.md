## 1

将运行中的 PoA（Proof of Authority，通常基于 Clique）私有链迁移到 PoS（Proof of Stake，即 The Merge 后的架构），是一个复杂且高风险的操作。**这不仅仅是修改配置，而是需要引入全新的共识层（Consensus Layer）客户端。**

目前的 Geth（v1.13.10）是执行层（Execution Layer, EL），你需要引入共识层（Consensus Layer, CL），如 Prysm、Lighthouse 或 Teku，并通过“合并（The Merge）”的方式完成转换。

以下是基于 **Geth (EL) + Prysm (CL)** 的详细实施步骤，旨在确保不丢失状态和日志。

-----

### ⚠️ 核心警告与准备

1.  **架构变更**：迁移后，节点将由两个进程组成：`Geth` (负责交易执行、智能合约、状态存储) + `Prysm` (负责 PoS 共识、出块逻辑)。
2.  **环境克隆**：**绝对不要直接在生产环境操作。** 请先将现有节点的 `chaindata` 和 `keystore` 复制到测试服务器，模拟演练成功后再对生产环境动手。

-----

### 第一阶段：准备工作

#### 1\. 安装必要工具

你需要下载 PoS 共识客户端（以 Prysm 为例）和创世块生成工具。

  * **Prysm**: [下载地址](https://github.com/prysmaticlabs/prysm/releases) (下载 `beacon-chain` 和 `validator`)
  * **Prysmctl**: 用于生成测试网创世数据。
  * **Eth2-testnet-genesis**: (可选，如果 prysmctl 不够用)

#### 2\. 生成 JWT Secret

EL 和 CL 之间通信需要身份验证。

```bash
openssl rand -hex 32 | tr -d "\n" > jwt.hex
```

#### 3\. 生成 PoS 验证者密钥 (Validator Keys)

PoS 需要质押 ETH（在私有链中不需要真实 ETH，但需要逻辑上的质押）。
使用 `prysmctl` 或 `eth2-val-tools` 生成验证者密钥库。你需要至少一个验证者来维持网络运行。

-----

### 第二阶段：配置执行层 (Geth)

你需要设定一个 **Terminal Total Difficulty (TTD)**。当链的总难度达到这个值时，网络将从 PoA 切换到 PoS。

#### 1\. 获取当前总难度

进入 Geth 控制台或通过 RPC 查询当前最新块的总难度：

```javascript
// Geth Console
eth.getBlock("latest").totalDifficulty
```

假设当前总难度是 `50000`。

#### 2\. 计算 TTD

由于 Clique PoA 的难度通常固定（例如 1 或 2），你需要设置一个比当前略高的值，以便你有时间启动 CL 客户端。

  * 建议 TTD = 当前总难度 + (预估由 PoA 产生的区块数 \* 2)。
  * 假设设定 TTD 为 `50050`。

#### 3\. 更新 Geth 配置

你需要覆盖当前的链配置（Genesis 配置）。**注意：不要使用 `geth init`，因为这会清除数据。** 你需要创建一个新的 `genesis.json` 用于重写配置，或者如果在 Geth 启动命令中使用了 `--config`，则修改该 TOML 文件。

**关键修改点**：在 `config` 字段下添加 `terminalTotalDifficulty`。

```json
{
  "config": {
    "chainId": 1234,,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "muirGlacierBlock": 0,
    "berlinBlock": 0,
    "londonBlock": 0,
    "arrowGlacierBlock": 0,
    "grayGlacierBlock": 0,
    "clique": {
      "period": 5,
      "epoch": 30000
    },
    "terminalTotalDifficulty": "50050"  <-- 必须添加此项，且必须是字符串
  },
  ...
}
```

**如何应用配置而不丢失数据？**
Geth 没有直接的命令来“更新”正在运行的链的创世配置。对于私有链，通常的做法是：

1.  停止 Geth。
2.  使用 `geth dumpconfig` 导出当前配置，修改加入 TTD。
3.  **高风险操作**：Geth 实际上会读取数据库中的配置。如果只是为了触发 Merge，最稳妥的方式是通过命令行参数 `--override.bedrock` (如果适用) 或者重新初始化一个拥有相同 `chainId` 但带有 TTD 的配置（需谨慎，通常不建议对已有数据 `init`）。
      * *更推荐的方法*：如果你的 Geth 是通过 `genesis.json` 初始化的，你需要确保 Geth 启动时识别到 TTD。在 v1.13.x 中，如果通过 `--config` 指定了 TOML 文件，修改 TOML 中的 `TerminalTotalDifficulty` 即可。

-----

### 第三阶段：生成共识层 (Beacon Chain) 创世状态

你需要生成 `genesis.ssz`，告诉共识层从哪里开始接管。

使用 `prysmctl` 工具：

```bash
./prysmctl testnet generate-genesis \
--fork=deneb \
--num-validators=1 \
--chain-id=1234 \
--output-ssz=genesis.ssz \
--execution-endpoint=http://localhost:8551 \
--jwt-secret=path/to/jwt.hex
```

  * `--fork`: Geth 1.13 支持较新的分叉，建议使用 `capella` 或 `deneb`。
  * `--num-validators`: 你的验证者数量。
  * **注意**：生成的 `genesis.ssz` 时间戳必须在未来，或者你需要确保 TTD 到达的时间与 Beacon Chain 启动的时间匹配。

-----

### 第四阶段：执行迁移 (The Merge)

#### 1\. 启动 Geth (执行层)

必须开启 Engine API 端口 (`8551`)。

```bash
geth \
  --datadir ./data \
  --http --http.api eth,net,web3,engine,admin \
  --authrpc.addr localhost \
  --authrpc.port 8551 \
  --authrpc.vhosts="*" \
  --authrpc.jwtsecret ./jwt.hex \
  --allow-insecure-unlock \
  --unlock "YOUR_ACCOUNT_ADDRESS" \
  --password ./password.txt \
  --mine \
  --miner.etherbase "YOUR_ACCOUNT_ADDRESS"
```

*注意：此时 Geth 仍在以 PoA 模式挖矿，直到总难度达到 TTD。*

#### 2\. 启动 Prysm Beacon Node (共识层)

```bash
./beacon-chain \
  --datadir=./consensus-data \
  --execution-endpoint=http://localhost:8551 \
  --jwt-secret=./jwt.hex \
  --genesis-state=./genesis.ssz \
  --min-sync-peers=0 \
  --chain-config-file=./config.yaml \
  --suggested-fee-recipient="YOUR_ETH_ADDRESS"
```

#### 3\. 启动 Prysm Validator (验证者)

```bash
./validator \
  --datadir=./consensus-data \
  --beacon-rpc-provider=localhost:4000 \
  --wallet-dir=./validator_keys \
  --wallet-password-file=./password.txt \
  --chain-config-file=./config.yaml
```

-----

### 第五阶段：观察与验证

1.  **监控 Geth 日志**：

      * 你会看到 PoA 仍在出块。
      * 一旦总难度达到 `50050` (你在配置中设定的 TTD)，Geth 日志会出现类似 `Reached terminal total difficulty` 的信息。
      * 此时，Geth 将停止 PoA 挖矿，转为等待 Engine API 的指令。

2.  **监控 Prysm 日志**：

      * Beacon Node 会显示 `Waiting for execution engine to reach TTD`。
      * 一旦 Geth 达到 TTD，Prysm 会接管并开始通过 Engine API 发送 `newPayload` 指令给 Geth。
      * 如果你看到 `Block produced` 或 `Attestation submitted`，说明 PoS 已经接管成功。

3.  **验证数据完整性**：

      * 连接 RPC，检查历史区块是否可读。
      * 检查账户余额和合约状态是否保持不变。

-----

### 关键技术细节与坑

  * **PoA 的难度增长**：Clique 共识中，如果签名者按顺序出块，难度增加 2；如果是乱序（out-of-turn），难度增加 1。计算 TTD 时要非常精确，或者在 Geth 启动后，手动发交易填满区块以加速难度增长达到 TTD。
  * **Bootnodes**：如果在多节点环境中，确保所有节点的 Geth 都更新了带有 TTD 的配置，并且所有节点都必须部署对应的 CL 客户端。
  * **时间同步**：PoS 对时间非常敏感，务必确保所有服务器使用了 NTP 同步时间。
  * **Geth 版本**：v1.13.10 已经去除了旧的 `puppet` 迁移工具，完全依赖 Engine API。确保你的 Prysm 版本也是最新的。

### 建议的后续步骤

由于该操作极其复杂且不可逆（除非恢复备份），**您是否需要我提供一份用于“本地演练”的 `docker-compose` 配置文件模板？** 这样您可以在不通过生产数据的情况下，先模拟一遍从 PoA 到 PoS 的全过程。

这是一个非常明智的决定。在对生产环境进行“心脏移植”（共识机制变更）之前，在一个沙盒环境中完整演练一遍是绝对必要的。

这份指南将帮助你搭建一个 **本地 Docker 模拟环境**。我们将模拟：

1.  启动一条 PoA 链。
2.  设定 TTD（Terminal Total Difficulty）。
3.  生成 PoS 创世数据。
4.  启动共识层（Prysm）并触发合并（The Merge）。

-----

### 目录结构准备

请在本地创建一个目录 `merge-simulation`，并在其中创建以下子目录：

```text
merge-simulation/
├── docker-compose.yml
├── config/
│   ├── genesis.json      (PoA 创世文件)
│   └── config.yaml       (Prysm 链配置)
├── data/                 (存放链数据，自动生成)
└── jwt.hex               (JWT 密钥)
```

#### 1\. 生成 JWT Secret

在终端运行：

```bash
openssl rand -hex 32 | tr -d "\n" > jwt.hex
```

-----

### 步骤 1: 配置文件

#### `config/genesis.json` (Geth 初始化配置)

这是一个标准的 Clique PoA 配置。注意 `terminalTotalDifficulty` 字段，这是触发合并的关键。

```json
{
  "config": {
    "chainId": 12345,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "muirGlacierBlock": 0,
    "berlinBlock": 0,
    "londonBlock": 0,
    "arrowGlacierBlock": 0,
    "grayGlacierBlock": 0,
    "clique": {
      "period": 5,
      "epoch": 30000
    },
    "terminalTotalDifficulty": "50" 
  },
  "difficulty": "1",
  "gasLimit": "8000000",
  "extradata": "0x000000000000000000000000000000000000000000000000000000000000000001234567890123456789012345678901234567890000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
  "alloc": {
    "0x123456789012345678901234567890123456789": { "balance": "1000000000000000000000" }
  }
}
```

> **注意**：这里 `terminalTotalDifficulty` 设为 `50`。这意味着当链产生大约 25-50 个块时（取决于出块者数量），合并就会触发。你需要根据你的测试速度调整这个值。对于新建测试链，50 很快就能达到。

#### `config/config.yaml` (Prysm 链配置)

定义 PoS 网络的基本参数。

```yaml
CONFIG_NAME: "local_testnet"

# Minimal presets for testing
PRESET_BASE: "minimal"

# Genesis
# ---------------------------------------------------------------
MIN_GENESIS_ACTIVE_VALIDATOR_COUNT: 1
MIN_GENESIS_TIME: 0
GENESIS_FORK_VERSION: 0x00000001

# Forking
# ---------------------------------------------------------------
ALTAIR_FORK_EPOCH: 0
BELLATRIX_FORK_EPOCH: 0
CAPELLA_FORK_EPOCH: 0
DENEB_FORK_EPOCH: 0

# Time parameters
# ---------------------------------------------------------------
SECONDS_PER_SLOT: 6
SLOTS_PER_EPOCH: 4

# Deposit contract
# ---------------------------------------------------------------
DEPOSIT_CHAIN_ID: 12345
DEPOSIT_NETWORK_ID: 12345
DEPOSIT_CONTRACT_ADDRESS: 0x123456789012345678901234567890123456789
```

-----

### 步骤 2: Docker Compose 配置

创建 `docker-compose.yml`。我们将利用 `prysmctl` 工具容器来生成密钥，然后启动节点。

```yaml
version: "3.8"

services:
  # 1. Geth 执行层节点
  geth:
    image: ethereum/client-go:v1.13.10
    container_name: el-client
    ports:
      - "8545:8545" 
      - "8551:8551"
    volumes:
      - ./config:/config
      - ./data/geth:/root/.ethereum
      - ./jwt.hex:/jwt.hex
    command: >
      --http 
      --http.api eth,net,engine,admin 
      --http.addr 0.0.0.0 
      --http.vhosts=* --http.corsdomain=*
      --authrpc.addr 0.0.0.0 
      --authrpc.port 8551 
      --authrpc.vhosts=* --authrpc.jwtsecret /jwt.hex 
      --networkid 12345 
      --mine 
      --miner.etherbase 0x123456789012345678901234567890123456789
      --allow-insecure-unlock 
      --nodiscover
      --syncmode full

  # 2. 创世数据生成工具 (运行一次即退出)
  create-beacon-chain-genesis:
    image: gcr.io/prysmaticlabs/prysm/cmd/prysmctl:latest
    container_name: genesis-gen
    volumes:
      - ./config:/config
      - ./data/consensus:/consensus-data
    command:
      - testnet
      - generate-genesis
      - --fork=deneb
      - --num-validators=1
      - --chain-config-file=/config/config.yaml
      - --output-ssz=/consensus-data/genesis.ssz
      - --start-time=0  # 0 表示立即开始 (或稍后基于当前时间)

  # 3. Prysm 信标节点 (共识层)
  beacon-chain:
    image: gcr.io/prysmaticlabs/prysm/beacon-chain:latest
    container_name: cl-beacon
    depends_on:
      geth:
        condition: service_started
      create-beacon-chain-genesis:
        condition: service_completed_successfully
    ports:
      - "4000:4000"
      - "3500:3500"
    volumes:
      - ./config:/config
      - ./data/consensus:/consensus-data
      - ./jwt.hex:/jwt.hex
    command:
      - --datadir=/consensus-data
      - --chain-config-file=/config/config.yaml
      - --genesis-state=/consensus-data/genesis.ssz
      - --execution-endpoint=http://el-client:8551
      - --jwt-secret=/jwt.hex
      - --min-sync-peers=0
      - --suggested-fee-recipient=0x123456789012345678901234567890123456789
      - --accept-terms-of-use

  # 4. Prysm 验证者节点
  validator:
    image: gcr.io/prysmaticlabs/prysm/validator:latest
    container_name: cl-validator
    depends_on:
      beacon-chain:
        condition: service_started
    volumes:
      - ./config:/config
      - ./data/consensus:/consensus-data
    command:
      - --datadir=/consensus-data
      - --chain-config-file=/config/config.yaml
      - --beacon-rpc-provider=cl-beacon:4000
      - --wallet-dir=/consensus-data/prysm-wallet-v1
      - --wallet-password-file=/consensus-data/password.txt
      - --accept-terms-of-use
```

-----

### 步骤 3: 演练操作流程

请严格按照以下顺序在终端执行：

#### 1\. 初始化 Geth 数据库

我们需要先初始化 Geth 的创世块。由于 docker-compose 会直接运行，我们需要手动先跑一次 init。

```bash
# 在 merge-simulation 目录下
docker run --rm -v $(pwd)/data/geth:/root/.ethereum -v $(pwd)/config:/config ethereum/client-go:v1.13.10 init /config/genesis.json
```

此时 `data/geth` 目录下应该生成了 `geth` 和 `keystore` 文件夹。

#### 2\. 生成验证者密钥

我们需要为 Prysm 生成验证者密钥。运行一个临时的 prysmctl 容器：

```bash
# 1. 创建密码文件
echo "password123" > data/consensus/password.txt

# 2. 运行生成工具 (注意挂载路径)
docker run --rm -v $(pwd)/data/consensus:/consensus-data gcr.io/prysmaticlabs/prysm/cmd/prysmctl:latest \
  validator recovery-phrase generate --mnemonic-language=english > mnemonic.txt

# 提取助记词 (假设你手动复制了上面命令输出的单词，这里为了演示简化，直接生成 keystore)
# 推荐：直接用 prysmctl 自动生成 keystore
docker run --rm -it \
  -v $(pwd)/data/consensus:/consensus-data \
  gcr.io/prysmaticlabs/prysm/cmd/prysmctl:latest \
  validator accounts create \
  --wallet-dir=/consensus-data/prysm-wallet-v1 \
  --wallet-password-file=/consensus-data/password.txt \
  --num-accounts=1
```

*(注：如果这一步报错，确保 data/consensus 文件夹存在)*

#### 3\. 启动整个网络

```bash
docker-compose up -d
```

#### 4\. 观察合并过程 (The Merge)

现在，最关键的一步来了。观察日志。

**查看 Geth 日志：**

```bash
docker logs -f el-client
```

你首先会看到 Geth 在以 PoA 模式挖矿（`Mined new block`）。
关注 `Difficulty` 的值。当它累积接近 `50` 时，你会看到类似信息：

> `Reached terminal total difficulty`

**查看 Beacon Chain 日志：**

```bash
docker logs -f cl-beacon
```

一开始它会报错说无法连接执行层（如果是启动初期），或者显示：

> `Waiting for execution engine to reach TTD`

**见证合并：**
一旦 Geth 达到 TTD 50：

1.  Geth 停止 PoA 挖矿。
2.  Prysm (CL) 开始接管，你会看到 `New payload` 或 `Forkchoice updated`。
3.  Validator 开始提议区块，你在 Geth 日志中会看到由 Prysm 推送过来的新块，而不是它自己挖的。
4.  你会看到一只熊猫（Prysm 的标志）或者绿色的 `Synced` 信息，表明合并成功。

-----

### 从模拟到生产的映射

在演练成功后，将此逻辑应用到你的生产环境时，请注意以下差异：

1.  **TTD 设置**：生产环境中，你需要计算 `当前总难度 + (未来几小时的区块数 * 难度增量)`。不要设得太近，给自己留出部署 Prysm 的时间。
2.  **数据迁移**：生产环境不能 `init`。你需要修改 `config.toml` 或者使用 `geth --override.bedrock` (如果适用) 来注入 TTD，或者修改源码重新编译（不推荐）。**最推荐的方法是：导出现有配置，添加 TTD，停止节点，用新配置启动。**
3.  **Prysm 配置**：生产环境不要使用 `minimal` preset，要使用 `mainnet` preset（即使是私有链，通常也基于主网参数修改），并且 `genesis.ssz` 需要根据生产环境的实际情况生成。

**下一步操作建议：**
请先尝试运行上述 Docker 环境。如果遇到报错（通常是密钥路径或权限问题），请告诉我具体的错误日志，我会帮你调整命令。如果跑通了，我们再讨论如何计算你生产环境的具体 TTD 数值。





## 3

这是一个非常专业且高风险的操作。你的链已经运行了半年，积累了宝贵的状态数据，因此**绝对不能**重新初始化（`geth init`），必须通过\*\*“软着陆”\*\*的方式，利用 TTD (Terminal Total Difficulty) 机制触发 Merge（合并）。

目前的架构是单纯的 **执行层 (Geth)**。
目标架构是 **执行层 (Geth) + 共识层 (Prysm Beacon + Validator)**。

以下是基于你现有 Docker 配置的详细迁移指南。

-----

### ⚠️ 第一步：备份（生死攸关）

在做任何操作之前，请停止所有容器，并备份整个项目目录（包含 `nodes/` 下的所有数据）。

```bash
docker-compose down
cp -r nodes nodes_backup_$(date +%F)
```

-----

### 第二步：准备工作与计算 TTD

你需要确定一个未来的“总难度值”，当链达到这个难度时，PoA 停止，PoS 接管。

1.  **查询当前状态**
    启动 `node0`（或者利用 RPC 端口）：

    ```bash
    # 如果节点没开，先启动
    docker-compose up -d node0
    ```

    进入容器查询当前区块号和总难度：

    ```bash
    docker exec -it node0 geth attach --exec "eth.getBlock('latest').totalDifficulty" /data/geth.ipc
    # 假设输出: 15000000
    
    docker exec -it node0 geth attach --exec "eth.blockNumber" /data/geth.ipc
    # 假设输出: 800000
    ```

2.  **计算 TTD**
    Clique PoA 机制下，正常出块难度增加 2，乱序出块难度增加 1。平均每个块增加难度 2。
    为了给你留出部署 Prysm 的时间（假设需要 1 小时，按 2秒/块计算，约 1800 个块），你需要设置一个未来的 TTD。

    **公式**：`目标 TTD = 当前总难度 + (预留区块数 * 2)`

    *示例*：
    当前难度 `15,000,000`。329248
    预留 3000 个块（约 1.5 小时）。
    目标 TTD = `15,000,000 + (3000 * 2)` = `15,006,000`。

    *请记下这个计算出的数字，下文用 `YOUR_CALCULATED_TTD` 代替。*

-----

### 第三步：生成共识层（Prysm）所需文件

你需要生成三个关键文件：

1.  `jwt.hex`：Geth 和 Prysm 通信的密钥。
2.  `genesis.ssz`：Prysm 的创世状态。
3.  `validator keys`：PoS 验证者的密钥。

在项目根目录下创建一个 `consensus` 文件夹用于存放这些临时生成的文件。

```bash
mkdir -p consensus/prysm-wallet
```

#### 1\. 生成 JWT Secret

```bash
openssl rand -hex 32 | tr -d "\n" > consensus/jwt.hex
```

#### 2\. 生成验证者密钥 (Keystore)

我们需要为你的 3 个挖矿节点（node0, node1, node2）生成对应的 PoS 验证者密钥。虽然是私有链，但也建议至少跑 1-3 个验证者。这里演示生成 3 个验证者密钥。

我们将使用 Docker 运行 `prysmctl` 来生成。

```bash
# 1. 创建钱包密码
echo "yourpassword" > consensus/password.txt

# 2. 生成验证者密钥 (生成3个)
docker run --rm -v $(pwd)/consensus:/consensus \
  gcr.io/prysmaticlabs/prysm/cmd/prysmctl:latest \
  validator accounts create \
  --wallet-dir=/consensus/prysm-wallet \
  --wallet-password-file=/consensus/password.txt \
  --num-accounts=3

# 此时 consensus/prysm-wallet 下会有 direct/accounts 目录

git clone https://github.com/ethereum/staking-deposit-cli.git
cd staking-deposit-cli
pip install -r requirements.txt
python deposit.py new-mnemonic --num_validators 3 --chain mainnet
```

#### 3\. 生成 Beacon Chain 创世文件 (genesis.ssz)

这是最关键的一步。我们需要告诉 Beacon Chain 如何启动，并且对接现有的 Geth 链 ID。

我们需要创建一个临时的 `config.yaml` 给 Prysm 用：
*创建文件 `consensus/config.yaml`*:

```yaml
CONFIG_NAME: "mainnet"
PRESET_BASE: "mainnet"
# 你的链 ID
DEPOSIT_CHAIN_ID: 20250521
DEPOSIT_NETWORK_ID: 20250521
# 随便填一个合约地址，因为我们是硬启动，不会走存币合约
DEPOSIT_CONTRACT_ADDRESS: 0x123456789012345678901234567890123456789
# 关键参数，根据私有链调整
SECONDS_PER_SLOT: 12
SLOTS_PER_EPOCH: 32
# 设置为 0 表示立即启动 (或根据需要设置未来时间戳)
MIN_GENESIS_TIME: 0
GENESIS_FORK_VERSION: 0x00000001
# 启用最新的分叉
ALTAIR_FORK_EPOCH: 0
BELLATRIX_FORK_EPOCH: 0
CAPELLA_FORK_EPOCH: 0
DENEB_FORK_EPOCH: 0
```

**生成 genesis.ssz**:
*注意：`--execution-endpoint` 需要能连通（或者先忽略检查），但在生成创世块时，重要的是 ChainID 和 Validator。*

```bash
docker run --rm -v $(pwd)/consensus:/consensus \
  gcr.io/prysmaticlabs/prysm/cmd/prysmctl:latest \
  testnet generate-genesis \
  --fork=deneb \
  --num-validators=3 \
  --chain-config-file=/consensus/config.yaml \
  --output-ssz=/consensus/genesis.ssz \
  --start-time=$(date +%s) # 使用当前时间作为 PoS 创世时间
```

-----

### 第四步：修改 Geth 配置文件

你现有的 Geth 是通过 `--config=/data/config.toml` 启动的。你需要修改 **每个节点** 目录下的 `config.toml` 文件（或者如果它们共享一个配置，就改那个）。

**重点修改项**：

1.  **开启 Engine API** (`authrpc`)。
2.  **设置 TTD**。

打开 `nodes/node0/data/config.toml` (以及 node1, node2...)，找到并修改/添加以下部分：

```toml
[Eth]
# ... 其他配置保持不变 ...

[Eth.Miner]
# ... 保持不变 ...

# 关键：覆盖创世配置中的 TTD
# 注意：这只有在 Geth 启动时读取 config.toml 才会生效
# 必须是字符串，填入你在第二步计算出的数值
OverrideTerminalTotalDifficulty = "YOUR_CALCULATED_TTD" 

[Node]
# ...
# 开启 Engine API 监听
AuthAddr = "0.0.0.0"
AuthPort = 8551
AuthVhosts = ["*"]
# JWT 路径 (我们稍后会挂载进去)
AuthSecret = "/jwt.hex"
```

**如果在 config.toml 中找不到 OverrideTerminalTotalDifficulty**：
你需要修改 `genesis.json` 中的 TTD，但这通常需要重置链。对于运行中的链，**推荐的方法**是在 Docker 的 `command` 中添加标志：
`--override.terminaltotaldifficulty=YOUR_CALCULATED_TTD` (注意：Geth 1.13 可能已经弃用此 flag，如果 config.toml 不生效，我们需要依赖 config.toml 的 `[Eth]` -\> `TerminalTotalDifficulty` 或者直接更新 Geth 内部状态，但在 Docker 环境下，利用 config.toml 是最稳妥的)。

*确认 Geth 1.13.10 的 config.toml 结构支持 `TerminalTotalDifficulty` 位于 `[Eth]` 字段下。*

-----

### 第五步：重写 Docker Compose

我们需要为每个**挖矿节点**配备一个 **Beacon Node** 和 **Validator**。
RPC 节点 (`node3`, `node4`) 只需要配备 Beacon Node 即可（不需要 Validator），以便它们能同步 PoS 区块。

为了简洁，这里只展示 **Node0 (Miner)** 和 **Node3 (RPC)** 的改造示例。你需要对 Node1 和 Node2 复制 Node0 的模式。

**docker-compose.yml 修改版：**

```yaml
version: '3'

networks:
  evm_net:
    driver: bridge
    ipam:
      config:
        - subnet: 172.16.238.0/24

services:
  # ================= Node 0 (Miner -> Validator) =================
  node0:
    image: ethereum/client-go:v1.13.10
    container_name: node0
    ports:
      - "30300:30300/tcp"
      - "30300:30300/udp"
      - "8551:8551" # 暴露 Engine API 用于调试，实际通信走内网
    volumes:
      - ./nodes/node0/data:/data
      - ./nodes/node0/keystore:/keystore
      - ./consensus/jwt.hex:/jwt.hex:ro  # 挂载 JWT
      - /env/evm/password:/password
      # 假设你已经更新了 config.toml 里的 TTD 和 Authrpc 配置
    command:
      - --config=/data/config.toml
      - --datadir=/data
      - --networkid=20250521
      - --syncmode=full
      - --mine
      - --miner.etherbase=0x23b6aef6ab0ed44d137256984a3fc8da7e9c79f9
      - --unlock=0x23b6aef6ab0ed44d137256984a3fc8da7e9c79f9
      - --password=/password
      - --allow-insecure-unlock
      - --nat=extip:172.16.238.10
      # 强制开启 Engine API 参数，以防 config.toml 没配好
      - --authrpc.addr=0.0.0.0
      - --authrpc.port=8551
      - --authrpc.vhosts=*
      - --authrpc.jwtsecret=/jwt.hex
    networks:
      evm_net:
        ipv4_address: 172.16.238.10

  # Node0 的共识客户端
  beacon0:
    image: gcr.io/prysmaticlabs/prysm/beacon-chain:v5.0.0 # 使用较新版本支持 Deneb
    container_name: beacon0
    depends_on: [node0]
    volumes:
      - ./consensus/data/node0:/data
      - ./consensus/jwt.hex:/jwt.hex:ro
      - ./consensus/genesis.ssz:/genesis.ssz:ro
      - ./consensus/config.yaml:/config.yaml:ro
    command:
      - --datadir=/data
      - --chain-config-file=/config.yaml
      - --genesis-state=/genesis.ssz
      - --execution-endpoint=http://172.16.238.10:8551 # 连接 node0
      - --jwt-secret=/jwt.hex
      - --accept-terms-of-use
      - --min-sync-peers=0 # 私有链不需要等待对等节点
      - --suggested-fee-recipient=0x23b6aef6ab0ed44d137256984a3fc8da7e9c79f9
      - --p2p-host-ip=172.16.238.20 # 给 beacon 分配一个 IP
    networks:
      evm_net:
        ipv4_address: 172.16.238.20

  # Node0 的验证者客户端
  validator0:
    image: gcr.io/prysmaticlabs/prysm/validator:v5.0.0
    container_name: validator0
    depends_on: [beacon0]
    volumes:
      - ./consensus/prysm-wallet:/wallet # 挂载生成的钱包
      - ./consensus/password.txt:/password.txt
      - ./consensus/config.yaml:/config.yaml:ro
    command:
      - --beacon-rpc-provider=172.16.238.20:4000 # 连接 beacon0
      - --wallet-dir=/wallet
      - --wallet-password-file=/password.txt
      - --chain-config-file=/config.yaml
      - --accept-terms-of-use
    networks:
      evm_net:
        ipv4_address: 172.16.238.30

  # ... node1, node2 依此类推，分别添加 beacon1, validator1 ...
  # 注意：如果只有一个钱包文件，validator0, validator1 需要挂载同一个 wallet 目录
  # 但通常建议分开，或者目前测试阶段只起一个 Validator0 也可以控制整个网络。

  # ================= Node 3 (RPC Node) =================
  node3:
    image: ethereum/client-go:v1.13.10
    container_name: node3
    # ... 你的原有端口配置 ...
    volumes:
      - ./nodes/node3/data:/data
      - ./nodes/node3/keystore:/keystore
      - ./consensus/jwt.hex:/jwt.hex:ro
    command:
      # ... 原有参数 ...
      - --config=/data/config.toml
      - --authrpc.addr=0.0.0.0
      - --authrpc.port=8551
      - --authrpc.vhosts=*
      - --authrpc.jwtsecret=/jwt.hex
      # 确保 RPC 节点开启全同步
      - --syncmode=full
    networks:
      evm_net:
        ipv4_address: 172.16.238.13

  # RPC 节点也需要 Beacon 才能同步 PoS 数据
  beacon3:
    image: gcr.io/prysmaticlabs/prysm/beacon-chain:v5.0.0
    container_name: beacon3
    depends_on: [node3]
    volumes:
      - ./consensus/data/node3:/data
      - ./consensus/jwt.hex:/jwt.hex:ro
      - ./consensus/genesis.ssz:/genesis.ssz:ro
      - ./consensus/config.yaml:/config.yaml:ro
    command:
      - --datadir=/data
      - --chain-config-file=/config.yaml
      - --genesis-state=/genesis.ssz
      - --execution-endpoint=http://172.16.238.13:8551
      - --jwt-secret=/jwt.hex
      - --accept-terms-of-use
      - --min-sync-peers=0
      - --p2p-host-ip=172.16.238.23
      - --peer=172.16.238.20 # 必须连接到 beacon0
    networks:
      evm_net:
        ipv4_address: 172.16.238.23
```

-----

### 第六步：实施迁移

1.  **更新配置**:
    确保所有 `nodeX/data/config.toml` 中都添加了 `OverrideTerminalTotalDifficulty = "你的计算值"`。
    确保 `jwt.hex` 已经分发或挂载到位。

2.  **启动网络**:

    ```bash
    docker-compose up -d
    ```

3.  **观察日志**:

      * **查看 Geth (node0)**:
        ```bash
        docker logs -f node0
        ```
        你会看到它继续出块。你需要确认日志中没有报错，并且 Authrpc 已经启动。
      * **查看 Beacon (beacon0)**:
        ```bash
        docker logs -f beacon0
        ```
        它应该会显示连接成功，并且提示 `Waiting for execution engine to reach TTD` (等待执行层达到总难度)。

4.  **触发 Merge**:
    如果你的 TTD 设置得比较远，你需要等待。如果设置得太近，可能启动时已经过了，这会导致不同步。

      * 如果觉得太慢，可以通过发送大量空交易来加速出块（增加难度）。
      * 一旦 Geth 的难度累计达到 TTD，Geth 会打印 `Reached terminal total difficulty`。
      * 此时，Geth 停止 Clique 挖矿。
      * Prysm 的 Validator 会开始提议区块，Beacon 会通过 Engine API 将新块传给 Geth。
      * Geth 日志中，出块信息将不再显示 `Mined`，而是显示 `Imported` 或 `New payload`，且 `coinbase` 应该是你在 Prysm 中设置的地址。

### 关键检查点

1.  **时间同步**: 确保宿主机时间准确，Docker 容器时间与宿主机一致。PoS 对时间极度敏感。
2.  **Fork ID**: 确保 `config.yaml` (Prysm) 中的 Chain ID 与 Geth 的 `networkid` 一致。
3.  **Shanghai/Cancun**: Geth v1.13 默认支持 Shanghai/Cancun。如果你的 PoA 链很老，请确保 Geth 的 config.toml 里 `ShanghaiTime` 和 `CancunTime` 设置为 `0` 或者某个过去的时间戳，或者与 Merge 发生时间一致，以确保 EVM 功能与 Prysm 的 Deneb 升级兼容。
    *建议在 config.toml 的 `[Eth.Config]` 下添加：*
    ```toml
    ShanghaiTime = 0
    CancunTime = 0
    ```
    这样可以确保 Geth 准备好处理 Prysm (Deneb) 发来的 Blob 相关的 payload。

这个方案能最大程度保证你的数据安全，因为我们完全复用了现有的 `data` 目录，只是改变了它的运行逻辑。祝迁移顺利！


## 居家节点搭建
