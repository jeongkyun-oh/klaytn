// Copyright 2018 The klaytn Authors
// Copyright 2015 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.
//
// This file is derived from cmd/utils/flags.go (2018/06/04).
// Modified and improved for the klaytn development.

package utils

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/ground-x/klaytn/accounts"
	"github.com/ground-x/klaytn/accounts/keystore"
	"github.com/ground-x/klaytn/api/debug"
	"github.com/ground-x/klaytn/blockchain"
	"github.com/ground-x/klaytn/blockchain/state"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/common/fdlimit"
	"github.com/ground-x/klaytn/crypto"
	"github.com/ground-x/klaytn/datasync/downloader"
	"github.com/ground-x/klaytn/metrics"
	"github.com/ground-x/klaytn/networks/p2p"
	"github.com/ground-x/klaytn/networks/p2p/discover"
	"github.com/ground-x/klaytn/networks/p2p/nat"
	"github.com/ground-x/klaytn/networks/p2p/netutil"
	"github.com/ground-x/klaytn/node"
	"github.com/ground-x/klaytn/node/cn"
	"github.com/ground-x/klaytn/node/sc"
	"github.com/ground-x/klaytn/params"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	CommandHelpTemplate = `{{.cmd.Name}}{{if .cmd.Subcommands}} command{{end}}{{if .cmd.Flags}} [command options]{{end}} [arguments...]
{{if .cmd.Description}}{{.cmd.Description}}
{{end}}{{if .cmd.Subcommands}}
SUBCOMMANDS:
	{{range .cmd.Subcommands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
	{{end}}{{end}}{{if .categorizedFlags}}
{{range $idx, $categorized := .categorizedFlags}}{{$categorized.Name}} OPTIONS:
{{range $categorized.Flags}}{{"\t"}}{{.}}
{{end}}
{{end}}{{end}}`
)

func init() {
	cli.AppHelpTemplate = `{{.Name}} {{if .Flags}}[global options] {{end}}command{{if .Flags}} [command options]{{end}} [arguments...]

VERSION:
  {{.Version}}

COMMANDS:
  {{range .Commands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
  {{end}}{{if .Flags}}
GLOBAL OPTIONS:
  {{range .Flags}}{{.}}
  {{end}}{{end}}
`

	cli.CommandHelpTemplate = CommandHelpTemplate
}

// NewApp creates an app with sane defaults.
func NewApp(gitCommit, usage string) *cli.App {
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Author = ""
	//app.Authors = nil
	app.Email = ""
	app.Version = params.Version
	if len(gitCommit) >= 8 {
		app.Version += "-" + gitCommit[:8]
	}
	app.Usage = usage
	return app
}

var (
	// General settings
	NetworkTypeFlag = cli.StringFlag{
		Name:  "networktype",
		Usage: "klaytn network type (main-net (mn), service chain-net (scn))",
		Value: "mn",
	}
	DbTypeFlag = cli.StringFlag{
		Name:  "dbtype",
		Usage: `Blockchain storage database type ("leveldb", "badger")`,
		Value: "leveldb",
	}
	SrvTypeFlag = cli.StringFlag{
		Name:  "srvtype",
		Usage: `json rpc server type ("http", "fasthttp")`,
		Value: "http",
	}
	DataDirFlag = DirectoryFlag{
		Name:  "datadir",
		Usage: "Data directory for the databases and keystore",
		Value: DirectoryString{node.DefaultDataDir()},
	}
	KeyStoreDirFlag = DirectoryFlag{
		Name:  "keystore",
		Usage: "Directory for the keystore (default = inside the datadir)",
	}
	// TODO-Klaytn-Bootnode: redefine networkid
	NetworkIdFlag = cli.Uint64Flag{
		Name:  "networkid",
		Usage: "Network identifier (integer, 1=Frontier, 2=Morden (disused), 3=Ropsten, 4=Rinkeby)",
		Value: cn.DefaultConfig.NetworkId,
	}
	IdentityFlag = cli.StringFlag{
		Name:  "identity",
		Usage: "Custom node name",
	}
	DocRootFlag = DirectoryFlag{
		Name:  "docroot",
		Usage: "Document Root for HTTPClient file scheme",
		Value: DirectoryString{homeDir()},
	}
	defaultSyncMode = cn.DefaultConfig.SyncMode
	SyncModeFlag    = TextMarshalerFlag{
		Name:  "syncmode",
		Usage: `Blockchain sync mode ("fast" or "full")`,
		Value: &defaultSyncMode,
	}
	GCModeFlag = cli.StringFlag{
		Name:  "gcmode",
		Usage: `Blockchain garbage collection mode ("full", "archive")`,
		Value: "full",
	}
	LightServFlag = cli.IntFlag{
		Name:  "lightserv",
		Usage: "Maximum percentage of time allowed for serving LES requests (0-90)",
		Value: 0,
	}
	LightPeersFlag = cli.IntFlag{
		Name:  "lightpeers",
		Usage: "Maximum number of LES client peers",
		Value: cn.DefaultConfig.LightPeers,
	}
	LightKDFFlag = cli.BoolFlag{
		Name:  "lightkdf",
		Usage: "Reduce key-derivation RAM & CPU usage at some expense of KDF strength",
	}
	// Transaction pool settings
	TxPoolNoLocalsFlag = cli.BoolFlag{
		Name:  "txpool.nolocals",
		Usage: "Disables price exemptions for locally submitted transactions",
	}
	TxPoolJournalFlag = cli.StringFlag{
		Name:  "txpool.journal",
		Usage: "Disk journal for local transaction to survive node restarts",
		Value: blockchain.DefaultTxPoolConfig.Journal,
	}
	TxPoolRejournalFlag = cli.DurationFlag{
		Name:  "txpool.rejournal",
		Usage: "Time interval to regenerate the local transaction journal",
		Value: blockchain.DefaultTxPoolConfig.Rejournal,
	}
	TxPoolPriceLimitFlag = cli.Uint64Flag{
		Name:  "txpool.pricelimit",
		Usage: "Minimum gas price limit to enforce for acceptance into the pool",
		Value: cn.DefaultConfig.TxPool.PriceLimit,
	}
	TxPoolPriceBumpFlag = cli.Uint64Flag{
		Name:  "txpool.pricebump",
		Usage: "Price bump percentage to replace an already existing transaction",
		Value: cn.DefaultConfig.TxPool.PriceBump,
	}
	TxPoolAccountSlotsFlag = cli.Uint64Flag{
		Name:  "txpool.accountslots",
		Usage: "Minimum number of executable transaction slots guaranteed per account",
		Value: cn.DefaultConfig.TxPool.AccountSlots,
	}
	TxPoolGlobalSlotsFlag = cli.Uint64Flag{
		Name:  "txpool.globalslots",
		Usage: "Maximum number of executable transaction slots for all accounts",
		Value: cn.DefaultConfig.TxPool.GlobalSlots,
	}
	TxPoolAccountQueueFlag = cli.Uint64Flag{
		Name:  "txpool.accountqueue",
		Usage: "Maximum number of non-executable transaction slots permitted per account",
		Value: cn.DefaultConfig.TxPool.AccountQueue,
	}
	TxPoolGlobalQueueFlag = cli.Uint64Flag{
		Name:  "txpool.globalqueue",
		Usage: "Maximum number of non-executable transaction slots for all accounts",
		Value: cn.DefaultConfig.TxPool.GlobalQueue,
	}
	TxPoolLifetimeFlag = cli.DurationFlag{
		Name:  "txpool.lifetime",
		Usage: "Maximum amount of time non-executable transaction are queued",
		Value: cn.DefaultConfig.TxPool.Lifetime,
	}
	// Performance tuning settings
	PartitionedDBFlag = cli.BoolFlag{
		Name:  "db.partitioned",
		Usage: "Use partitioned databases or single database for persistent storage",
	}
	LevelDBCacheSizeFlag = cli.IntFlag{
		Name:  "db.leveldb.cache-size",
		Usage: "Size of in-memory cache in LevelDB (MiB)",
		Value: 768,
	}
	DBParallelDBWriteFlag = cli.BoolFlag{
		Name:  "db.parallel-write",
		Usage: "Determines parallel or serial writes of block data to database",
	}
	TrieMemoryCacheSizeFlag = cli.IntFlag{
		Name:  "state.cache-size",
		Usage: "Size of in-memory cache of the global state (in MiB) to flush matured singleton trie nodes to disk",
		Value: 256,
	}
	TrieCacheGenFlag = cli.IntFlag{
		Name:  "state.cache-gens",
		Usage: "Number of the global state generations to keep in memory",
		Value: int(state.MaxTrieCacheGen),
	}
	TrieBlockIntervalFlag = cli.UintFlag{
		Name:  "state.block-interval",
		Usage: "An interval in terms of block number to commit the global state to disk",
		Value: blockchain.DefaultBlockInterval,
	}
	CacheTypeFlag = cli.IntFlag{
		Name:  "cache.type",
		Usage: "Cache Type: 0=LRUCache, 1=LRUShardCache, 2=FIFOCache",
		Value: int(common.DefaultCacheType),
	}
	CacheScaleFlag = cli.IntFlag{
		Name:  "cache.scale",
		Usage: "Scale of cache (cache size = preset size * scale of cache(%))",
	}
	CacheUsageLevelFlag = cli.StringFlag{
		Name:  "cache.level",
		Usage: "Set the cache usage level ('saving', 'normal', 'extreme')",
	}
	MemorySizeFlag = cli.IntFlag{
		Name:  "cache.memory",
		Usage: "Set the physical RAM size (GB, Default: 16GB)",
	}
	CacheWriteThroughFlag = cli.BoolFlag{
		Name:  "cache.writethrough",
		Usage: "Enables write-through writing to database and cache for certain types of cache.",
	}
	ChildChainIndexingFlag = cli.BoolFlag{
		Name:  "childchainindexing",
		Usage: "Enables storing transaction hash of child chain transaction for fast access to child chain data",
	}
	// Miner settings
	MiningEnabledFlag = cli.BoolFlag{
		Name:  "mine",
		Usage: "Enable mining",
	}
	MinerThreadsFlag = cli.IntFlag{
		Name:  "minerthreads",
		Usage: "Number of CPU threads to use for mining",
		Value: runtime.NumCPU(),
	}
	TargetGasLimitFlag = cli.Uint64Flag{
		Name:  "targetgaslimit",
		Usage: "Target gas limit sets the artificial target gas floor for the blocks to mine",
		Value: params.GenesisGasLimit,
	}
	CoinbaseFlag = cli.StringFlag{
		Name:  "coinbase",
		Usage: "Public address for block mining rewards (default = first account created)",
		Value: "0",
	}
	RewardbaseFlag = cli.StringFlag{
		Name:  "rewardbase",
		Usage: "Public address for block consensus rewards (default = first account created)",
		Value: "0",
	}
	RewardContractFlag = cli.StringFlag{
		Name:  "rewardcontract",
		Usage: "Public address for rewards contract",
		Value: "0",
	}
	// TODO-Klaytn-Issue136 default gasPrice
	GasPriceFlag = BigFlag{
		Name:  "gasprice",
		Usage: "Minimal gas price to accept for mining a transactions",
		Value: cn.DefaultConfig.GasPrice,
	}
	ExtraDataFlag = cli.StringFlag{
		Name:  "extradata",
		Usage: "Block extra data set by the work (default = client version)",
	}
	// Account settings
	UnlockedAccountFlag = cli.StringFlag{
		Name:  "unlock",
		Usage: "Comma separated list of accounts to unlock",
		Value: "",
	}
	PasswordFileFlag = cli.StringFlag{
		Name:  "password",
		Usage: "Password file to use for non-interactive password input",
		Value: "",
	}

	VMEnableDebugFlag = cli.BoolFlag{
		Name:  "vmdebug",
		Usage: "Record information useful for VM and contract debugging",
	}
	VMLogTargetFlag = cli.IntFlag{
		Name:  "vmlog",
		Usage: "Set the output target of vmlog precompiled contract (0: no output, 1: file, 2: stdout, 3: both)",
		Value: 0,
	}

	// Logging and debug settings
	MetricsEnabledFlag = cli.BoolFlag{
		Name:  metrics.MetricsEnabledFlag,
		Usage: "Enable metrics collection and reporting",
	}
	PrometheusExporterFlag = cli.BoolFlag{
		Name:  metrics.PrometheusExporterFlag,
		Usage: "Enable prometheus exporter",
	}
	PrometheusExporterPortFlag = cli.IntFlag{
		Name:  metrics.PrometheusExporterPortFlag,
		Usage: "Prometheus exporter listening port",
		Value: 61001,
	}
	// RPC settings
	RPCEnabledFlag = cli.BoolFlag{
		Name:  "rpc",
		Usage: "Enable the HTTP-RPC server",
	}
	RPCListenAddrFlag = cli.StringFlag{
		Name:  "rpcaddr",
		Usage: "HTTP-RPC server listening interface",
		Value: node.DefaultHTTPHost,
	}
	RPCPortFlag = cli.IntFlag{
		Name:  "rpcport",
		Usage: "HTTP-RPC server listening port",
		Value: node.DefaultHTTPPort,
	}
	RPCCORSDomainFlag = cli.StringFlag{
		Name:  "rpccorsdomain",
		Usage: "Comma separated list of domains from which to accept cross origin requests (browser enforced)",
		Value: "",
	}
	RPCVirtualHostsFlag = cli.StringFlag{
		Name:  "rpcvhosts",
		Usage: "Comma separated list of virtual hostnames from which to accept requests (server enforced). Accepts '*' wildcard.",
		Value: strings.Join(node.DefaultConfig.HTTPVirtualHosts, ","),
	}
	RPCApiFlag = cli.StringFlag{
		Name:  "rpcapi",
		Usage: "API's offered over the HTTP-RPC interface",
		Value: "",
	}
	IPCDisabledFlag = cli.BoolFlag{
		Name:  "ipcdisable",
		Usage: "Disable the IPC-RPC server",
	}
	IPCPathFlag = DirectoryFlag{
		Name:  "ipcpath",
		Usage: "Filename for IPC socket/pipe within the datadir (explicit paths escape it)",
	}
	WSEnabledFlag = cli.BoolFlag{
		Name:  "ws",
		Usage: "Enable the WS-RPC server",
	}
	WSListenAddrFlag = cli.StringFlag{
		Name:  "wsaddr",
		Usage: "WS-RPC server listening interface",
		Value: node.DefaultWSHost,
	}
	WSPortFlag = cli.IntFlag{
		Name:  "wsport",
		Usage: "WS-RPC server listening port",
		Value: node.DefaultWSPort,
	}
	GRPCEnabledFlag = cli.BoolFlag{
		Name:  "grpc",
		Usage: "Enable the gRPC server",
	}
	GRPCListenAddrFlag = cli.StringFlag{
		Name:  "grpcaddr",
		Usage: "gRPC server listening interface",
		Value: node.DefaultGRPCHost,
	}
	GRPCPortFlag = cli.IntFlag{
		Name:  "grpcport",
		Usage: "gRPC server listening port",
		Value: node.DefaultGRPCPort,
	}
	WSApiFlag = cli.StringFlag{
		Name:  "wsapi",
		Usage: "API's offered over the WS-RPC interface",
		Value: "",
	}
	WSAllowedOriginsFlag = cli.StringFlag{
		Name:  "wsorigins",
		Usage: "Origins from which to accept websockets requests",
		Value: "",
	}
	ExecFlag = cli.StringFlag{
		Name:  "exec",
		Usage: "Execute JavaScript statement",
	}
	PreloadJSFlag = cli.StringFlag{
		Name:  "preload",
		Usage: "Comma separated list of JavaScript files to preload into the console",
	}

	// Network Settings
	NodeTypeFlag = cli.StringFlag{
		Name:  "nodetype",
		Usage: "klaytn node type (consensus node (cn), proxy node (pn), endpoint node (en))",
		Value: "en",
	}
	MaxPeersFlag = cli.IntFlag{
		Name:  "maxpeers",
		Usage: "Maximum number of network peers (network disabled if set to 0)",
		Value: 25,
	}
	MaxPendingPeersFlag = cli.IntFlag{
		Name:  "maxpendpeers",
		Usage: "Maximum number of pending connection attempts (defaults used if set to 0)",
		Value: 0,
	}
	ListenPortFlag = cli.IntFlag{
		Name:  "port",
		Usage: "Network listening port",
		Value: 30303,
	}
	SubListenPortFlag = cli.IntFlag{
		Name:  "subport",
		Usage: "Network sub listening port",
		Value: 30304,
	}
	MultiChannelUseFlag = cli.BoolFlag{
		Name:  "multichannel",
		Usage: "Create a dedicated channel for block propagation",
	}
	BootnodesFlag = cli.StringFlag{
		Name:  "bootnodes",
		Usage: "Comma separated kni URLs for P2P discovery bootstrap (set v4+v5 instead for light servers)",
		Value: "",
	}
	BootnodesV4Flag = cli.StringFlag{
		Name:  "bootnodesv4",
		Usage: "Comma separated kni URLs for P2P v4 discovery bootstrap (light server, full nodes)",
		Value: "",
	}
	// TODO-Klaytn-Bootnode: decide porting or not ethereum's node discovery V5
	/*
		BootnodesV5Flag = cli.StringFlag{
			Name:  "bootnodesv5",
			Usage: "Comma separated kni URLs for P2P v5 discovery bootstrap (light server, light nodes)",
			Value: "",
		}
	*/
	NodeKeyFileFlag = cli.StringFlag{
		Name:  "nodekey",
		Usage: "P2P node key file",
	}
	NodeKeyHexFlag = cli.StringFlag{
		Name:  "nodekeyhex",
		Usage: "P2P node key as hex (for testing)",
	}
	NATFlag = cli.StringFlag{
		Name:  "nat",
		Usage: "NAT port mapping mechanism (any|none|upnp|pmp|extip:<IP>)",
		Value: "any",
	}
	NoDiscoverFlag = cli.BoolFlag{
		Name:  "nodiscover",
		Usage: "Disables the peer discovery mechanism (manual peer addition)",
	}
	NetrestrictFlag = cli.StringFlag{
		Name:  "netrestrict",
		Usage: "Restricts network communication to the given IP network (CIDR masks)",
	}
	ChainAccountAddrFlag = cli.StringFlag{
		Name:  "chainaddr",
		Usage: "A hex account address in parent chain used to sign service chain transaction",
	}
	AnchoringPeriodFlag = cli.Uint64Flag{
		Name:  "chaintxperiod",
		Usage: "The period to make and send a chain transaction to parent chain",
		Value: 1,
	}
	SentChainTxsLimit = cli.Uint64Flag{
		Name:  "chaintxlimit",
		Usage: "Number of service chain transactions stored for resending",
		Value: 100,
	}

	// ATM the url is left to the user and deployment to
	JSpathFlag = cli.StringFlag{
		Name:  "jspath",
		Usage: "JavaScript root path for `loadScript`",
		Value: ".",
	}

	// Baobab bootnodes setting
	BaobabFlag = cli.BoolFlag{
		Name:  "baobab",
		Usage: "Pre-configured Klaytn baobab network",
	}
	//TODO-Klaytn-Node remove after the real bootnode is implemented
	EnableSBNFlag = cli.BoolFlag{
		Name:  "enableSBN",
		Usage: "enable simple bootnodes in order to retrieve two PNs' URIs",
	}
	// Bootnode's settings
	AddrFlag = cli.StringFlag{
		Name:  "addr",
		Usage: `udp listen port`,
		Value: "32323",
	}
	GenKeyFlag = cli.StringFlag{
		Name:  "genkey",
		Usage: "generate a node private key and write to given filename",
	}
	WriteAddressFlag = cli.BoolFlag{
		Name:  "writeaddress",
		Usage: `write out the node's public key which is given by "--nodekeyfile" or "--nodekeyhex"`,
	}
	// gennodekey flags
	GenNodeKeyToFileFlag = cli.BoolFlag{
		Name:  "file",
		Usage: `Generate a nodekey and a klaytn node information as files`,
	}
	GenNodeKeyPortFlag = cli.IntFlag{
		Name:  "port",
		Usage: `Specify a tcp port number`,
		Value: 32323,
	}
	GenNodeKeyIPFlag = cli.StringFlag{
		Name:  "ip",
		Usage: `Specify an ip address`,
		Value: "0.0.0.0",
	}
	// ServiceChain's settings
	EnabledBridgeFlag = cli.BoolFlag{
		Name:  "bridge",
		Usage: "Enable bridge service for service chain",
	}
	IsMainBridgeFlag = cli.BoolFlag{
		Name:  "mainbridge",
		Usage: "Enable bridge as main bridge",
	}
	BridgeListenPortFlag = cli.IntFlag{
		Name:  "bridgeport",
		Usage: "bridge listen port",
		Value: 50505,
	}
	ParentChainURLFlag = cli.StringFlag{
		Name:  "parentchainws",
		Usage: "parentchain ws url",
		Value: "ws://0.0.0.0:8546",
	}

	// TODO-Klaytn-Bootnode: Add bootnode's metric options
	// TODO-Klaytn-Bootnode: Implements bootnode's RPC
)

// MakeDataDir retrieves the currently requested data directory, terminating
// if none (or the empty string) is specified. If the node is starting a baobab,
// the a subdirectory of the specified datadir will be used.
func MakeDataDir(ctx *cli.Context) string {
	if path := ctx.GlobalString(DataDirFlag.Name); path != "" {
		if ctx.GlobalBool(BaobabFlag.Name) {
			return filepath.Join(path, "baobab")
		}
		return path
	}
	Fatalf("Cannot determine default data directory, please set manually (--datadir)")
	return ""
}

// setNodeKey creates a node key from set command line flags, either loading it
// from a file or as a specified hex value. If neither flags were provided, this
// method returns nil and an emphemeral key is to be generated.
func setNodeKey(ctx *cli.Context, cfg *p2p.Config) {
	var (
		hex  = ctx.GlobalString(NodeKeyHexFlag.Name)
		file = ctx.GlobalString(NodeKeyFileFlag.Name)
		key  *ecdsa.PrivateKey
		err  error
	)
	switch {
	case file != "" && hex != "":
		Fatalf("Options %q and %q are mutually exclusive", NodeKeyFileFlag.Name, NodeKeyHexFlag.Name)
	case file != "":
		if key, err = crypto.LoadECDSA(file); err != nil {
			Fatalf("Option %q: %v", NodeKeyFileFlag.Name, err)
		}
		cfg.PrivateKey = key
	case hex != "":
		if key, err = crypto.HexToECDSA(hex); err != nil {
			Fatalf("Option %q: %v", NodeKeyHexFlag.Name, err)
		}
		cfg.PrivateKey = key
	}
}

// setNodeUserIdent creates the user identifier from CLI flags.
func setNodeUserIdent(ctx *cli.Context, cfg *node.Config) {
	if identity := ctx.GlobalString(IdentityFlag.Name); len(identity) > 0 {
		cfg.UserIdent = identity
	}
}

// setBootstrapNodes creates a list of bootstrap nodes from the command line
// flags, reverting to pre-configured ones if none have been specified.
func setBootstrapNodes(ctx *cli.Context, cfg *p2p.Config) {
	urls := params.MainnetBootnodes
	switch {
	case ctx.GlobalIsSet(BootnodesFlag.Name) || ctx.GlobalIsSet(BootnodesV4Flag.Name):
		if ctx.GlobalIsSet(BootnodesV4Flag.Name) {
			urls = strings.Split(ctx.GlobalString(BootnodesV4Flag.Name), ",")
		} else {
			urls = strings.Split(ctx.GlobalString(BootnodesFlag.Name), ",")
		}
	case ctx.GlobalIsSet(BaobabFlag.Name):
		// set pre-configured bootnodes when 'baobab' option was enabled
		urls = getBaobabBootnodesByConnectionType(int(cfg.ConnectionType))
	case cfg.BootstrapNodes != nil:
		return // already set, don't apply defaults.
	}

	cfg.BootstrapNodes = make([]*discover.Node, 0, len(urls))
	for _, url := range urls {
		node, err := discover.ParseNode(url)
		if err != nil {
			logger.Error("Bootstrap URL invalid", "kni", url, "err", err)
			continue
		}
		cfg.BootstrapNodes = append(cfg.BootstrapNodes, node)
	}
}

// setListenAddress creates a TCP listening address string from set command
// line flags.
func setListenAddress(ctx *cli.Context, cfg *p2p.Config) {
	if ctx.GlobalIsSet(ListenPortFlag.Name) {
		cfg.ListenAddr = fmt.Sprintf(":%d", ctx.GlobalInt(ListenPortFlag.Name))
	}

	if ctx.GlobalBool(MultiChannelUseFlag.Name) {
		cfg.EnableMultiChannelServer = true
		if ctx.GlobalIsSet(SubListenPortFlag.Name) {
			cfg.SubListenAddr = nil
			SubListenAddr := fmt.Sprintf(":%d", ctx.GlobalInt(SubListenPortFlag.Name))
			cfg.SubListenAddr = append(cfg.SubListenAddr, SubListenAddr)
		}
	}
}

// setNAT creates a port mapper from command line flags.
func setNAT(ctx *cli.Context, cfg *p2p.Config) {
	if ctx.GlobalIsSet(NATFlag.Name) {
		natif, err := nat.Parse(ctx.GlobalString(NATFlag.Name))
		if err != nil {
			Fatalf("Option %s: %v", NATFlag.Name, err)
		}
		cfg.NAT = natif
	}
}

// splitAndTrim splits input separated by a comma
// and trims excessive white space from the substrings.
func splitAndTrim(input string) []string {
	result := strings.Split(input, ",")
	for i, r := range result {
		result[i] = strings.TrimSpace(r)
	}
	return result
}

// setHTTP creates the HTTP RPC listener interface string from the set
// command line flags, returning empty if the HTTP endpoint is disabled.
func setHTTP(ctx *cli.Context, cfg *node.Config) {
	if ctx.GlobalBool(RPCEnabledFlag.Name) && cfg.HTTPHost == "" {
		cfg.HTTPHost = "127.0.0.1"
		if ctx.GlobalIsSet(RPCListenAddrFlag.Name) {
			cfg.HTTPHost = ctx.GlobalString(RPCListenAddrFlag.Name)
		}
	}

	if ctx.GlobalIsSet(RPCPortFlag.Name) {
		cfg.HTTPPort = ctx.GlobalInt(RPCPortFlag.Name)
	}
	if ctx.GlobalIsSet(RPCCORSDomainFlag.Name) {
		cfg.HTTPCors = splitAndTrim(ctx.GlobalString(RPCCORSDomainFlag.Name))
	}
	if ctx.GlobalIsSet(RPCApiFlag.Name) {
		cfg.HTTPModules = splitAndTrim(ctx.GlobalString(RPCApiFlag.Name))
	}
	if ctx.GlobalIsSet(RPCVirtualHostsFlag.Name) {
		cfg.HTTPVirtualHosts = splitAndTrim(ctx.GlobalString(RPCVirtualHostsFlag.Name))
	}
}

// setWS creates the WebSocket RPC listener interface string from the set
// command line flags, returning empty if the HTTP endpoint is disabled.
func setWS(ctx *cli.Context, cfg *node.Config) {
	if ctx.GlobalBool(WSEnabledFlag.Name) && cfg.WSHost == "" {
		cfg.WSHost = "127.0.0.1"
		if ctx.GlobalIsSet(WSListenAddrFlag.Name) {
			cfg.WSHost = ctx.GlobalString(WSListenAddrFlag.Name)
		}
	}

	if ctx.GlobalIsSet(WSPortFlag.Name) {
		cfg.WSPort = ctx.GlobalInt(WSPortFlag.Name)
	}
	if ctx.GlobalIsSet(WSAllowedOriginsFlag.Name) {
		cfg.WSOrigins = splitAndTrim(ctx.GlobalString(WSAllowedOriginsFlag.Name))
	}
	if ctx.GlobalIsSet(WSApiFlag.Name) {
		cfg.WSModules = splitAndTrim(ctx.GlobalString(WSApiFlag.Name))
	}
}

// setIPC creates an IPC path configuration from the set command line flags,
// returning an empty string if IPC was explicitly disabled, or the set path.
func setIPC(ctx *cli.Context, cfg *node.Config) {
	checkExclusive(ctx, IPCDisabledFlag, IPCPathFlag)
	switch {
	case ctx.GlobalBool(IPCDisabledFlag.Name):
		cfg.IPCPath = ""
	case ctx.GlobalIsSet(IPCPathFlag.Name):
		cfg.IPCPath = ctx.GlobalString(IPCPathFlag.Name)
	}
}

// setgRPC creates the gRPC listener interface string from the set
// command line flags, returning empty if the gRPC endpoint is disabled.
func setgRPC(ctx *cli.Context, cfg *node.Config) {
	if ctx.GlobalBool(GRPCEnabledFlag.Name) && cfg.GRPCHost == "" {
		cfg.GRPCHost = "127.0.0.1"
		if ctx.GlobalIsSet(GRPCListenAddrFlag.Name) {
			cfg.GRPCHost = ctx.GlobalString(GRPCListenAddrFlag.Name)
		}
	}

	if ctx.GlobalIsSet(GRPCPortFlag.Name) {
		cfg.GRPCPort = ctx.GlobalInt(GRPCPortFlag.Name)
	}
}

// makeDatabaseHandles raises out the number of allowed file handles per process
// for Geth and returns half of the allowance to assign to the database.
func makeDatabaseHandles() int {
	limit, err := fdlimit.Current()
	if err != nil {
		Fatalf("Failed to retrieve file descriptor allowance: %v", err)
	}
	if limit < 2048 {
		if err := fdlimit.Raise(2048); err != nil {
			Fatalf("Failed to raise file descriptor allowance: %v", err)
		}
	}
	if limit > 2048 { // cap database file descriptors even if more is available
		limit = 2048
	}
	return limit / 2 // Leave half for networking and other stuff
}

// MakeAddress converts an account specified directly as a hex encoded string or
// a key index in the key store to an internal account representation.
func MakeAddress(ks *keystore.KeyStore, account string) (accounts.Account, error) {
	// If the specified account is a valid address, return it
	if common.IsHexAddress(account) {
		return accounts.Account{Address: common.HexToAddress(account)}, nil
	}
	// Otherwise try to interpret the account as a keystore index
	index, err := strconv.Atoi(account)
	if err != nil || index < 0 {
		return accounts.Account{}, fmt.Errorf("invalid account address or index %q", account)
	}
	logger.Warn("Use explicit addresses! Referring to accounts by order in the keystore folder is dangerous and will be deprecated!")

	accs := ks.Accounts()
	if len(accs) <= index {
		return accounts.Account{}, fmt.Errorf("index %d higher than number of accounts %d", index, len(accs))
	}
	return accs[index], nil
}

// setCoinbase retrieves the coinbase either from the directly specified
// command line flags or from the keystore if CLI indexed.
func setGxbase(ctx *cli.Context, ks *keystore.KeyStore, cfg *cn.Config) {
	if ctx.GlobalIsSet(CoinbaseFlag.Name) {
		account, err := MakeAddress(ks, ctx.GlobalString(CoinbaseFlag.Name))
		if err != nil {
			Fatalf("Option %q: %v", CoinbaseFlag.Name, err)
		}
		cfg.Gxbase = account.Address
	}
}

// setRewardbase retrieves the rewardbase either from the directly specified
// command line flags or from the keystore if CLI indexed.
func setRewardbase(ctx *cli.Context, ks *keystore.KeyStore, cfg *cn.Config) {
	if ctx.GlobalIsSet(RewardbaseFlag.Name) {
		account, err := MakeAddress(ks, ctx.GlobalString(RewardbaseFlag.Name))
		if err != nil {
			Fatalf("Option %q: %v", RewardbaseFlag.Name, err)
		}
		cfg.Rewardbase = account.Address
	}
}

// setRewardbase retrieves the rewardbase either from the directly specified
// command line flags or from the keystore if CLI indexed.
func setRewardContract(ctx *cli.Context, cfg *cn.Config) {
	if ctx.GlobalIsSet(RewardContractFlag.Name) {
		cfg.RewardContract = common.HexToAddress(ctx.GlobalString(RewardContractFlag.Name))
	}
}

// MakePasswordList reads password lines from the file specified by the global --password flag.
func MakePasswordList(ctx *cli.Context) []string {
	path := ctx.GlobalString(PasswordFileFlag.Name)
	if path == "" {
		return nil
	}
	text, err := ioutil.ReadFile(path)
	if err != nil {
		Fatalf("Failed to read password file: %v", err)
	}
	lines := strings.Split(string(text), "\n")
	// Sanitise DOS line endings.
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	return lines
}

func SetP2PConfig(ctx *cli.Context, cfg *p2p.Config) {
	setNodeKey(ctx, cfg)
	setNAT(ctx, cfg)
	setListenAddress(ctx, cfg)

	lightServer := ctx.GlobalInt(LightServFlag.Name) != 0
	lightPeers := ctx.GlobalInt(LightPeersFlag.Name)

	var nodeType string
	if ctx.GlobalIsSet(NodeTypeFlag.Name) {
		nodeType = ctx.GlobalString(NodeTypeFlag.Name)
	} else {
		nodeType = NodeTypeFlag.Value
	}

	cfg.ConnectionType = convertNodeType(nodeType)
	if cfg.ConnectionType == node.UNKNOWNNODE {
		logger.Crit("Unknown node type", "nodetype", nodeType)
	}
	logger.Info("Setting connection type", "nodetype", nodeType, "conntype", cfg.ConnectionType)

	// set bootnodes via this function by check specified parameters
	setBootstrapNodes(ctx, cfg)

	if ctx.GlobalIsSet(MaxPeersFlag.Name) {
		cfg.MaxPeers = ctx.GlobalInt(MaxPeersFlag.Name)
		if lightServer && !ctx.GlobalIsSet(LightPeersFlag.Name) {
			cfg.MaxPeers += lightPeers
		}
	} else {
		if lightServer {
			cfg.MaxPeers += lightPeers
		}
	}
	if !(lightServer) {
		lightPeers = 0
	}
	ethPeers := cfg.MaxPeers - lightPeers
	logger.Info("Maximum peer count", "KLAY", ethPeers, "LES", lightPeers, "total", cfg.MaxPeers)

	if ctx.GlobalIsSet(MaxPendingPeersFlag.Name) {
		cfg.MaxPendingPeers = ctx.GlobalInt(MaxPendingPeersFlag.Name)
	}
	if ctx.GlobalIsSet(NoDiscoverFlag.Name) {
		cfg.NoDiscovery = true
	}
	//TODO-Klaytn-Node remove after the real bootnode is implemented
	if ctx.GlobalIsSet(EnableSBNFlag.Name) {
		cfg.EnableSBN = true
	}

	if netrestrict := ctx.GlobalString(NetrestrictFlag.Name); netrestrict != "" {
		list, err := netutil.ParseNetlist(netrestrict)
		if err != nil {
			Fatalf("Option %q: %v", NetrestrictFlag.Name, err)
		}
		cfg.NetRestrict = list
	}

}

func convertNodeType(nodetype string) p2p.ConnType {
	switch strings.ToLower(nodetype) {
	case "cn":
		return node.CONSENSUSNODE
	case "pn":
		return node.PROXYNODE
	case "en":
		return node.ENDPOINTNODE
	default:
		return node.UNKNOWNNODE
	}
}

func convertNodeTypeToString(nodetype int) string {
	switch p2p.ConnType(nodetype) {
	case node.CONSENSUSNODE:
		return "CN"
	case node.PROXYNODE:
		return "PN"
	case node.ENDPOINTNODE:
		return "EN"
	default:
		logger.Error("failed to convert nodetype as string", "err", "unknown nodetype")
		return "unknown"
	}
}

// SetNodeConfig applies node-related command line flags to the config.
func SetNodeConfig(ctx *cli.Context, cfg *node.Config) {
	SetP2PConfig(ctx, &cfg.P2P)
	setIPC(ctx, cfg)

	// httptype is http or fasthttp
	if ctx.GlobalIsSet(SrvTypeFlag.Name) {
		cfg.HTTPServerType = ctx.GlobalString(SrvTypeFlag.Name)
	}

	setHTTP(ctx, cfg)
	setWS(ctx, cfg)
	setgRPC(ctx, cfg)
	setNodeUserIdent(ctx, cfg)

	// dbtype is leveldb or badger
	if ctx.GlobalIsSet(DbTypeFlag.Name) {
		cfg.DBType = ctx.GlobalString(DbTypeFlag.Name)
	}

	if ctx.GlobalIsSet(DataDirFlag.Name) {
		cfg.DataDir = ctx.GlobalString(DataDirFlag.Name)
	}

	if ctx.GlobalIsSet(KeyStoreDirFlag.Name) {
		cfg.KeyStoreDir = ctx.GlobalString(KeyStoreDirFlag.Name)
	}
	if ctx.GlobalIsSet(LightKDFFlag.Name) {
		cfg.UseLightweightKDF = ctx.GlobalBool(LightKDFFlag.Name)
	}
}

func setTxPool(ctx *cli.Context, cfg *blockchain.TxPoolConfig) {
	if ctx.GlobalIsSet(TxPoolNoLocalsFlag.Name) {
		cfg.NoLocals = ctx.GlobalBool(TxPoolNoLocalsFlag.Name)
	}
	if ctx.GlobalIsSet(TxPoolJournalFlag.Name) {
		cfg.Journal = ctx.GlobalString(TxPoolJournalFlag.Name)
	}
	if ctx.GlobalIsSet(TxPoolRejournalFlag.Name) {
		cfg.Rejournal = ctx.GlobalDuration(TxPoolRejournalFlag.Name)
	}
	if ctx.GlobalIsSet(TxPoolPriceLimitFlag.Name) {
		cfg.PriceLimit = ctx.GlobalUint64(TxPoolPriceLimitFlag.Name)
	}
	if ctx.GlobalIsSet(TxPoolPriceBumpFlag.Name) {
		cfg.PriceBump = ctx.GlobalUint64(TxPoolPriceBumpFlag.Name)
	}
	if ctx.GlobalIsSet(TxPoolAccountSlotsFlag.Name) {
		cfg.AccountSlots = ctx.GlobalUint64(TxPoolAccountSlotsFlag.Name)
	}
	if ctx.GlobalIsSet(TxPoolGlobalSlotsFlag.Name) {
		cfg.GlobalSlots = ctx.GlobalUint64(TxPoolGlobalSlotsFlag.Name)
	}
	if ctx.GlobalIsSet(TxPoolAccountQueueFlag.Name) {
		cfg.AccountQueue = ctx.GlobalUint64(TxPoolAccountQueueFlag.Name)
	}
	if ctx.GlobalIsSet(TxPoolGlobalQueueFlag.Name) {
		cfg.GlobalQueue = ctx.GlobalUint64(TxPoolGlobalQueueFlag.Name)
	}
	if ctx.GlobalIsSet(TxPoolLifetimeFlag.Name) {
		cfg.Lifetime = ctx.GlobalDuration(TxPoolLifetimeFlag.Name)
	}
}

// checkExclusive verifies that only a single instance of the provided flags was
// set by the user. Each flag might optionally be followed by a string type to
// specialize it further.
func checkExclusive(ctx *cli.Context, args ...interface{}) {
	set := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		// Make sure the next argument is a flag and skip if not set
		flag, ok := args[i].(cli.Flag)
		if !ok {
			panic(fmt.Sprintf("invalid argument, not cli.Flag type: %T", args[i]))
		}
		// Check if next arg extends current and expand its name if so
		name := flag.GetName()

		if i+1 < len(args) {
			switch option := args[i+1].(type) {
			case string:
				// Extended flag, expand the name and shift the arguments
				if ctx.GlobalString(flag.GetName()) == option {
					name += "=" + option
				}
				i++

			case cli.Flag:
			default:
				panic(fmt.Sprintf("invalid argument, not cli.Flag or string extension: %T", args[i+1]))
			}
		}
		// Mark the flag if it's set
		if ctx.GlobalIsSet(flag.GetName()) {
			set = append(set, "--"+name)
		}
	}
	if len(set) > 1 {
		Fatalf("Flags %v can't be used at the same time", strings.Join(set, ", "))
	}
}

// SetKlayConfig applies klay-related command line flags to the config.
func SetKlayConfig(ctx *cli.Context, stack *node.Node, cfg *cn.Config) {
	// TODO-Klaytn-Bootnode: better have to check conflicts about network flags when we add Klaytn's `mainnet` parameter
	// checkExclusive(ctx, DeveloperFlag, TestnetFlag, RinkebyFlag)

	ks := stack.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
	setGxbase(ctx, ks, cfg)
	setRewardbase(ctx, ks, cfg)
	setRewardContract(ctx, cfg)
	setTxPool(ctx, &cfg.TxPool)

	if ctx.GlobalIsSet(SyncModeFlag.Name) {
		cfg.SyncMode = *GlobalTextMarshaler(ctx, SyncModeFlag.Name).(*downloader.SyncMode)
	}
	if ctx.GlobalIsSet(LightServFlag.Name) {
		cfg.LightServ = ctx.GlobalInt(LightServFlag.Name)
	}
	if ctx.GlobalIsSet(LightPeersFlag.Name) {
		cfg.LightPeers = ctx.GlobalInt(LightPeersFlag.Name)
	}
	if ctx.GlobalIsSet(NetworkIdFlag.Name) {
		cfg.NetworkId = ctx.GlobalUint64(NetworkIdFlag.Name)
	}

	if ctx.GlobalIsSet(PartitionedDBFlag.Name) {
		cfg.PartitionedDB = true
	}
	if ctx.GlobalIsSet(LevelDBCacheSizeFlag.Name) {
		cfg.LevelDBCacheSize = ctx.GlobalInt(LevelDBCacheSizeFlag.Name)
	}
	cfg.DatabaseHandles = makeDatabaseHandles()

	if gcmode := ctx.GlobalString(GCModeFlag.Name); gcmode != "full" && gcmode != "archive" {
		Fatalf("--%s must be either 'full' or 'archive'", GCModeFlag.Name)
	}
	cfg.NoPruning = ctx.GlobalString(GCModeFlag.Name) == "archive"
	logger.Info("Archiving mode of this node", "isArchiveMode", cfg.NoPruning)

	// TODO-Klaytn-ServiceChain Add human-readable address once its implementation is introduced.
	if ctx.GlobalIsSet(ChainAccountAddrFlag.Name) {
		tempStr := ctx.GlobalString(ChainAccountAddrFlag.Name)
		if !common.IsHexAddress(tempStr) {
			logger.Crit("Given chainaddr does not meet hex format.", "chainaddr", tempStr)
		}
		tempAddr := common.StringToAddress(tempStr)
		cfg.ChainAccountAddr = &tempAddr
		logger.Info("A chain address is registered.", "chainAccountAddr", *cfg.ChainAccountAddr)
	}
	cfg.AnchoringPeriod = ctx.GlobalUint64(AnchoringPeriodFlag.Name)
	cfg.SentChainTxsLimit = ctx.GlobalUint64(SentChainTxsLimit.Name)

	if ctx.GlobalIsSet(TrieMemoryCacheSizeFlag.Name) {
		cfg.TrieCacheSize = ctx.GlobalInt(TrieMemoryCacheSizeFlag.Name)
	}
	if ctx.GlobalIsSet(CacheTypeFlag.Name) {
		common.DefaultCacheType = common.CacheType(ctx.GlobalInt(CacheTypeFlag.Name))
	}
	if ctx.GlobalIsSet(TrieBlockIntervalFlag.Name) {
		cfg.TrieBlockInterval = ctx.GlobalUint(TrieBlockIntervalFlag.Name)
	}
	if ctx.GlobalIsSet(CacheScaleFlag.Name) {
		common.CacheScale = ctx.GlobalInt(CacheScaleFlag.Name)
	}
	if ctx.GlobalIsSet(CacheUsageLevelFlag.Name) {
		cacheUsageLevelFlag := ctx.GlobalString(CacheUsageLevelFlag.Name)
		if scaleByCacheUsageLevel, err := common.GetScaleByCacheUsageLevel(cacheUsageLevelFlag); err != nil {
			logger.Crit("Incorrect CacheUsageLevelFlag value", "error", err, "CacheUsageLevelFlag", cacheUsageLevelFlag)
		} else {
			common.ScaleByCacheUsageLevel = scaleByCacheUsageLevel
		}
	}
	if ctx.GlobalIsSet(MemorySizeFlag.Name) {
		physicalMemory := common.TotalPhysicalMemGB
		common.TotalPhysicalMemGB = ctx.GlobalInt(MemorySizeFlag.Name)
		logger.Info("Physical memory has been replaced by user settings", "PhysicalMemory(GB)", physicalMemory, "UserSetting(GB)", common.TotalPhysicalMemGB)
	} else {
		logger.Debug("Memory settings", "PhysicalMemory(GB)", common.TotalPhysicalMemGB)
	}
	if ctx.GlobalIsSet(CacheWriteThroughFlag.Name) {
		common.WriteThroughCaching = ctx.GlobalBool(CacheWriteThroughFlag.Name)
	}
	if ctx.GlobalIsSet(MinerThreadsFlag.Name) {
		cfg.MinerThreads = ctx.GlobalInt(MinerThreadsFlag.Name)
	}
	if ctx.GlobalIsSet(DocRootFlag.Name) {
		cfg.DocRoot = ctx.GlobalString(DocRootFlag.Name)
	}
	if ctx.GlobalIsSet(ExtraDataFlag.Name) {
		cfg.ExtraData = []byte(ctx.GlobalString(ExtraDataFlag.Name))
	}
	if ctx.GlobalIsSet(ChildChainIndexingFlag.Name) {
		cfg.ChildChainIndexing = true
	}
	if ctx.GlobalIsSet(DBParallelDBWriteFlag.Name) {
		cfg.ParallelDBWrite = ctx.GlobalBool(DBParallelDBWriteFlag.Name)
	}

	// TODO-Klaytn-RemoveLater Later we have to remove GasPriceFlag, because we disable user configurable gasPrice
	/*
		if ctx.GlobalIsSet(GasPriceFlag.Name) {
			cfg.GasPrice = GlobalBig(ctx, GasPriceFlag.Name) // TODO-Klaytn-Issue136 gasPrice
		}
	*/
	if ctx.GlobalIsSet(VMEnableDebugFlag.Name) {
		// TODO(fjl): force-enable this in --dev mode
		cfg.EnablePreimageRecording = ctx.GlobalBool(VMEnableDebugFlag.Name)
	}
	if ctx.GlobalIsSet(VMLogTargetFlag.Name) {
		if _, err := debug.Handler.SetVMLogTarget(ctx.GlobalInt(VMLogTargetFlag.Name)); err != nil {
			logger.Warn("Incorrect vmlog value", "err", err)
		}
	}

	// Override any default configs for hard coded network.
	// TODO-Klaytn-Bootnode: Discuss and add `baobab` test network's genesis block
	/*
		if ctx.GlobalBool(TestnetFlag.Name) {
			if !ctx.GlobalIsSet(NetworkIdFlag.Name) {
				cfg.NetworkId = 3
			}
			cfg.Genesis = blockchain.DefaultTestnetGenesisBlock()
		}
	*/
	// TODO(fjl): move trie cache generations into config
	if gen := ctx.GlobalInt(TrieCacheGenFlag.Name); gen > 0 {
		state.MaxTrieCacheGen = uint16(gen)
	}
}

// RegisterCNService adds a CN client to the stack.
func RegisterCNService(stack *node.Node, cfg *cn.Config) {
	// TODO-Klaytn add syncMode.LightSync func and add LesServer
	err := stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
		cfg.WsEndpoint = stack.WSEndpoint()
		fullNode, err := cn.New(ctx, cfg)
		return fullNode, err
	})
	if err != nil {
		Fatalf("Failed to register the CN service: %v", err)
	}
}

// RegisterServiceChainService adds a ServiceChain node to the stack.
func RegisterServiceChainService(stack *node.Node, cfg *cn.Config) {
	err := stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
		cfg.WsEndpoint = stack.WSEndpoint()
		fullNode, err := cn.NewServiceChain(ctx, cfg)
		return fullNode, err
	})
	if err != nil {
		Fatalf("Failed to register the SCN service: %v", err)
	}
}

func RegisterService(stack *node.Node, cfg *sc.SCConfig) {
	if cfg.EnabledBridge {
		err := stack.RegisterSubService(func(ctx *node.ServiceContext) (node.Service, error) {
			if cfg.IsMainBridge {
				mainBridge, err := sc.NewMainBridge(ctx, cfg)
				return mainBridge, err
			} else {
				subBridge, err := sc.NewSubBridge(ctx, cfg)
				return subBridge, err
			}
		})
		if err != nil {
			Fatalf("Failed to register the service: %v", err)
		}
	}
}

// SetupNetwork configures the system for either the main net or some test network.
func SetupNetwork(ctx *cli.Context) {
	// TODO(fjl): move target gas limit into config
	params.TargetGasLimit = ctx.GlobalUint64(TargetGasLimitFlag.Name)
}

func IsParallelDBWrite(ctx *cli.Context) bool {
	pw := false
	if ctx.GlobalIsSet(DBParallelDBWriteFlag.Name) {
		pw = ctx.GlobalBool(DBParallelDBWriteFlag.Name)
	}
	return pw
}

func IsPartitionedDB(ctx *cli.Context) bool {
	if ctx.GlobalIsSet(PartitionedDBFlag.Name) {
		return true
	}
	return false
}

// MakeConsolePreloads retrieves the absolute paths for the console JavaScript
// scripts to preload before starting.
func MakeConsolePreloads(ctx *cli.Context) []string {
	// Skip preloading if there's nothing to preload
	if ctx.GlobalString(PreloadJSFlag.Name) == "" {
		return nil
	}
	// Otherwise resolve absolute paths and return them
	preloads := []string{}

	assets := ctx.GlobalString(JSpathFlag.Name)
	for _, file := range strings.Split(ctx.GlobalString(PreloadJSFlag.Name), ",") {
		preloads = append(preloads, common.AbsolutePath(assets, strings.TrimSpace(file)))
	}
	return preloads
}

// MigrateFlags sets the global flag from a local flag when it's set.
// This is a temporary function used for migrating old command/flags to the
// new format.
//
// e.g. klay account new --keystore /tmp/mykeystore --lightkdf
//
// is equivalent after calling this method with:
//
// klay --keystore /tmp/mykeystore --lightkdf account new
//
// This allows the use of the existing configuration functionality.
// When all flags are migrated this function can be removed and the existing
// configuration functionality must be changed that is uses local flags
func MigrateFlags(action func(ctx *cli.Context) error) func(*cli.Context) error {
	return func(ctx *cli.Context) error {
		for _, name := range ctx.FlagNames() {
			if ctx.IsSet(name) {
				ctx.GlobalSet(name, ctx.String(name))
			}
		}
		return action(ctx)
	}
}

func getBaobabBootnodesByConnectionType(cType int) []string {
	if cType >= int(node.CONSENSUSNODE) && cType <= int(node.PROXYNODE) {
		return params.BaobabBootnodes[cType].Addrs
	}
	logger.Crit("Does not have any bootnode of given node type", "node_type", convertNodeTypeToString(cType))
	return []string{}
}
