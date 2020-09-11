// Modifications Copyright 2018 The klaytn Authors
// Copyright 2017 The go-ethereum Authors
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
// This file is derived from cmd/geth/config.go (2018/06/04).
// Modified and improved for the klaytn development.

package nodecmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"unicode"

	"github.com/klaytn/klaytn/cmd/utils"
	"github.com/klaytn/klaytn/datasync/chaindatafetcher"
	"github.com/klaytn/klaytn/datasync/dbsyncer"
	"github.com/klaytn/klaytn/log"
	"github.com/klaytn/klaytn/node"
	"github.com/klaytn/klaytn/node/cn"
	"github.com/klaytn/klaytn/node/sc"
	"github.com/klaytn/klaytn/params"
	"gopkg.in/urfave/cli.v1"

	"io"

	"github.com/naoina/toml"
)

// These settings ensure that TOML keys use the same names as Go struct fields.
var tomlSettings = toml.Config{
	NormFieldName: func(rt reflect.Type, key string) string {
		return key
	},
	FieldToKey: func(rt reflect.Type, field string) string {
		return field
	},
	MissingField: func(rt reflect.Type, field string) error {
		link := ""
		if unicode.IsUpper(rune(rt.Name()[0])) && rt.PkgPath() != "main" {
			link = fmt.Sprintf(", see https://godoc.org/%s#%s for available fields", rt.PkgPath(), rt.Name())
		}
		return fmt.Errorf("field '%s' is not defined in %s%s", field, rt.String(), link)
	},
}

type klayConfig struct {
	CN   cn.Config
	Node node.Config
}

// GetDumpConfigCommand returns cli.Command `dumpconfig` whose flags are initialized with nodeFlags and rpcFlags.
func GetDumpConfigCommand(nodeFlags, rpcFlags []cli.Flag) cli.Command {
	return cli.Command{
		Action:      utils.MigrateFlags(dumpConfig),
		Name:        "dumpconfig",
		Usage:       "Show configuration values",
		ArgsUsage:   "",
		Flags:       append(append(nodeFlags, rpcFlags...)),
		Category:    "MISCELLANEOUS COMMANDS",
		Description: `The dumpconfig command shows configuration values.`,
	}
}

func loadConfig(file string, cfg *klayConfig) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	err = tomlSettings.NewDecoder(bufio.NewReader(f)).Decode(cfg)
	// Add file name to errors that have a line number.
	if _, ok := err.(*toml.LineError); ok {
		err = errors.New(file + ", " + err.Error())
	}
	return err
}

func defaultNodeConfig() node.Config {
	cfg := node.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = params.VersionWithCommit(gitCommit)
	cfg.HTTPModules = append(cfg.HTTPModules, "klay", "shh")
	cfg.WSModules = append(cfg.WSModules, "klay", "shh")
	cfg.IPCPath = "klay.ipc"
	return cfg
}

func makeConfigNode(ctx *cli.Context) (*node.Node, klayConfig) {
	// Load defaults.
	cfg := klayConfig{
		CN:   *cn.GetDefaultConfig(),
		Node: defaultNodeConfig(),
	}

	// Load config file.
	if file := ctx.GlobalString(utils.ConfigFileFlag.Name); file != "" {
		if err := loadConfig(file, &cfg); err != nil {
			log.Fatalf("%v", err)
		}
	}

	// Apply flags.
	utils.SetNodeConfig(ctx, &cfg.Node)
	stack, err := node.New(&cfg.Node)
	if err != nil {
		log.Fatalf("Failed to create the protocol stack: %v", err)
	}
	utils.SetKlayConfig(ctx, stack, &cfg.CN)

	//utils.SetShhConfig(ctx, stack, &cfg.Shh)
	//utils.SetDashboardConfig(ctx, &cfg.Dashboard)

	return stack, cfg
}

func makeChainDataFetcherConfig(ctx *cli.Context) chaindatafetcher.ChainDataFetcherConfig {
	cfg := chaindatafetcher.DefaultChainDataFetcherConfig

	if ctx.GlobalBool(utils.EnableChainDataFetcherFlag.Name) {
		cfg.EnabledChainDataFetcher = true

		if ctx.GlobalIsSet(utils.ChainDataFetcherNoDefault.Name) {
			cfg.NoDefaultStart = true
		}
		if ctx.GlobalIsSet(utils.ChainDataFetcherNumHandlers.Name) {
			cfg.NumHandlers = ctx.GlobalInt(utils.ChainDataFetcherNumHandlers.Name)
		}
		if ctx.GlobalIsSet(utils.ChainDataFetcherJobChannelSize.Name) {
			cfg.JobChannelSize = ctx.GlobalInt(utils.ChainDataFetcherJobChannelSize.Name)
		}
		if ctx.GlobalIsSet(utils.ChainDataFetcherChainEventSizeFlag.Name) {
			cfg.BlockChannelSize = ctx.GlobalInt(utils.ChainDataFetcherChainEventSizeFlag.Name)
		}

		//kasConfig := kas.DefaultKASConfig
		//if ctx.GlobalIsSet(utils.ChainDataFetcherKASDBHostFlag.Name) {
		//	dbhost := ctx.GlobalString(utils.ChainDataFetcherKASDBHostFlag.Name)
		//	kasConfig.DBHost = dbhost
		//} else {
		//	logger.Crit("DBHost must be set !", "key", utils.ChainDataFetcherKASDBHostFlag.Name)
		//}
		//if ctx.GlobalIsSet(utils.ChainDataFetcherKASDBPortFlag.Name) {
		//	dbport := ctx.GlobalString(utils.ChainDataFetcherKASDBPortFlag.Name)
		//	kasConfig.DBPort = dbport
		//}
		//if ctx.GlobalIsSet(utils.ChainDataFetcherKASDBUserFlag.Name) {
		//	dbuser := ctx.GlobalString(utils.ChainDataFetcherKASDBUserFlag.Name)
		//	kasConfig.DBUser = dbuser
		//} else {
		//	logger.Crit("DBUser must be set !", "key", utils.ChainDataFetcherKASDBUserFlag.Name)
		//}
		//if ctx.GlobalIsSet(utils.ChainDataFetcherKASDBPasswordFlag.Name) {
		//	dbpasswd := ctx.GlobalString(utils.ChainDataFetcherKASDBPasswordFlag.Name)
		//	kasConfig.DBPassword = dbpasswd
		//} else {
		//	logger.Crit("DBPassword must be set !", "key", utils.ChainDataFetcherKASDBPasswordFlag.Name)
		//}
		//if ctx.GlobalIsSet(utils.ChainDataFetcherKASDBNameFlag.Name) {
		//	dbname := ctx.GlobalString(utils.ChainDataFetcherKASDBNameFlag.Name)
		//	kasConfig.DBName = dbname
		//} else {
		//	logger.Crit("DBName must be set !", "key", utils.ChainDataFetcherKASDBNameFlag.Name)
		//}
		//
		//if ctx.GlobalBool(utils.ChainDataFetcherKASCacheUse.Name) {
		//	kasConfig.CacheUse = true
		//	if ctx.GlobalIsSet(utils.ChainDataFetcherKASCacheURLFlag.Name) {
		//		cacheInvalidationUrl := ctx.GlobalString(utils.ChainDataFetcherKASCacheURLFlag.Name)
		//		kasConfig.CacheInvalidationURL = cacheInvalidationUrl
		//	} else {
		//		logger.Crit("The cache invalidation url is not set")
		//	}
		//	if ctx.GlobalIsSet(utils.ChainDataFetcherKASBasicAuthParamFlag.Name) {
		//		auth := ctx.GlobalString(utils.ChainDataFetcherKASBasicAuthParamFlag.Name)
		//		kasConfig.BasicAuthParam = auth
		//	} else {
		//		logger.Crit("The authorization is not set")
		//	}
		//	if ctx.GlobalIsSet(utils.ChainDataFetcherKASXChainIdFlag.Name) {
		//		xchainid := ctx.GlobalString(utils.ChainDataFetcherKASXChainIdFlag.Name)
		//		kasConfig.XChainId = xchainid
		//	} else {
		//		logger.Crit("The x-chain-id is not set")
		//	}
		//}
		//cfg.KasConfig = kasConfig
	}

	return *cfg
}

func makeDBSyncerConfig(ctx *cli.Context) dbsyncer.DBConfig {
	cfg := dbsyncer.DefaultDBConfig

	if ctx.GlobalBool(utils.EnableDBSyncerFlag.Name) {
		cfg.EnabledDBSyncer = true

		if ctx.GlobalIsSet(utils.DBHostFlag.Name) {
			dbhost := ctx.GlobalString(utils.DBHostFlag.Name)
			cfg.DBHost = dbhost
		} else {
			logger.Crit("DBHost must be set !", "key", utils.DBHostFlag.Name)
		}
		if ctx.GlobalIsSet(utils.DBPortFlag.Name) {
			dbports := ctx.GlobalString(utils.DBPortFlag.Name)
			cfg.DBPort = dbports
		}
		if ctx.GlobalIsSet(utils.DBUserFlag.Name) {
			dbuser := ctx.GlobalString(utils.DBUserFlag.Name)
			cfg.DBUser = dbuser
		} else {
			logger.Crit("DBUser must be set !", "key", utils.DBUserFlag.Name)
		}
		if ctx.GlobalIsSet(utils.DBPasswordFlag.Name) {
			dbpasswd := ctx.GlobalString(utils.DBPasswordFlag.Name)
			cfg.DBPassword = dbpasswd
		} else {
			logger.Crit("DBPassword must be set !", "key", utils.DBPasswordFlag.Name)
		}
		if ctx.GlobalIsSet(utils.DBNameFlag.Name) {
			dbname := ctx.GlobalString(utils.DBNameFlag.Name)
			cfg.DBName = dbname
		} else {
			logger.Crit("DBName must be set !", "key", utils.DBNameFlag.Name)
		}
		if ctx.GlobalBool(utils.EnabledLogModeFlag.Name) {
			cfg.EnabledLogMode = true
		}
		if ctx.GlobalIsSet(utils.MaxIdleConnsFlag.Name) {
			cfg.MaxIdleConns = ctx.GlobalInt(utils.MaxIdleConnsFlag.Name)
		}
		if ctx.GlobalIsSet(utils.MaxOpenConnsFlag.Name) {
			cfg.MaxOpenConns = ctx.GlobalInt(utils.MaxOpenConnsFlag.Name)
		}
		if ctx.GlobalIsSet(utils.ConnMaxLifeTimeFlag.Name) {
			cfg.ConnMaxLifetime = ctx.GlobalDuration(utils.ConnMaxLifeTimeFlag.Name)
		}
		if ctx.GlobalIsSet(utils.DBSyncerModeFlag.Name) {
			cfg.Mode = strings.ToLower(ctx.GlobalString(utils.DBSyncerModeFlag.Name))
		}
		if ctx.GlobalIsSet(utils.GenQueryThreadFlag.Name) {
			cfg.GenQueryThread = ctx.GlobalInt(utils.GenQueryThreadFlag.Name)
		}
		if ctx.GlobalIsSet(utils.InsertThreadFlag.Name) {
			cfg.InsertThread = ctx.GlobalInt(utils.InsertThreadFlag.Name)
		}
		if ctx.GlobalIsSet(utils.BulkInsertSizeFlag.Name) {
			cfg.BulkInsertSize = ctx.GlobalInt(utils.BulkInsertSizeFlag.Name)
		}
		if ctx.GlobalIsSet(utils.EventModeFlag.Name) {
			cfg.EventMode = strings.ToLower(ctx.GlobalString(utils.EventModeFlag.Name))
		}
		if ctx.GlobalIsSet(utils.MaxBlockDiffFlag.Name) {
			cfg.MaxBlockDiff = ctx.GlobalUint64(utils.MaxBlockDiffFlag.Name)
		}
		if ctx.GlobalIsSet(utils.BlockSyncChannelSizeFlag.Name) {
			cfg.BlockChannelSize = ctx.GlobalInt(utils.BlockSyncChannelSizeFlag.Name)
		}
	}

	return *cfg
}

func makeServiceChainConfig(ctx *cli.Context) (config sc.SCConfig) {
	cfg := sc.DefaultConfig

	// bridge service
	if ctx.GlobalBool(utils.MainBridgeFlag.Name) {
		cfg.EnabledMainBridge = true
		cfg.MainBridgePort = fmt.Sprintf(":%d", ctx.GlobalInt(utils.MainBridgeListenPortFlag.Name))
	}

	if ctx.GlobalBool(utils.SubBridgeFlag.Name) {
		cfg.EnabledSubBridge = true
		cfg.SubBridgePort = fmt.Sprintf(":%d", ctx.GlobalInt(utils.SubBridgeListenPortFlag.Name))
	}

	cfg.Anchoring = ctx.GlobalBool(utils.ServiceChainAnchoringFlag.Name)
	cfg.ChildChainIndexing = ctx.GlobalIsSet(utils.ChildChainIndexingFlag.Name)
	cfg.AnchoringPeriod = ctx.GlobalUint64(utils.AnchoringPeriodFlag.Name)
	cfg.SentChainTxsLimit = ctx.GlobalUint64(utils.SentChainTxsLimit.Name)
	cfg.ParentChainID = ctx.GlobalUint64(utils.ParentChainIDFlag.Name)
	cfg.VTRecovery = ctx.GlobalBool(utils.VTRecoveryFlag.Name)
	cfg.VTRecoveryInterval = ctx.GlobalUint64(utils.VTRecoveryIntervalFlag.Name)
	cfg.ServiceChainConsensus = utils.ServiceChainConsensusFlag.Value

	cfg.KASAnchor = ctx.GlobalBool(utils.KASServiceChainAnchorFlag.Name)
	if cfg.KASAnchor {
		cfg.KASAnchorPeriod = ctx.GlobalUint64(utils.KASServiceChainAnchorPeriodFlag.Name)
		if cfg.KASAnchorPeriod == 0 {
			cfg.KASAnchorPeriod = 1
			logger.Warn("KAS anchor period is set by 1")
		}

		cfg.KASAnchorUrl = ctx.GlobalString(utils.KASServiceChainAnchorUrlFlag.Name)
		if cfg.KASAnchorUrl == "" {
			logger.Crit("KAS anchor url should be set", "key", utils.KASServiceChainAnchorUrlFlag.Name)
		}

		cfg.KASAnchorOperator = ctx.GlobalString(utils.KASServiceChainAnchorOperatorFlag.Name)
		if cfg.KASAnchorOperator == "" {
			logger.Crit("KAS anchor operator should be set", "key", utils.KASServiceChainAnchorOperatorFlag.Name)
		}

		cfg.KASAccessKey = ctx.GlobalString(utils.KASServiceChainAccessKeyFlag.Name)
		if cfg.KASAccessKey == "" {
			logger.Crit("KAS access key should be set", "key", utils.KASServiceChainAccessKeyFlag.Name)
		}

		cfg.KASSecretKey = ctx.GlobalString(utils.KASServiceChainSecretKeyFlag.Name)
		if cfg.KASSecretKey == "" {
			logger.Crit("KAS secret key should be set", "key", utils.KASServiceChainSecretKeyFlag.Name)
		}

		cfg.KASXChainId = ctx.GlobalString(utils.KASServiceChainXChainIdFlag.Name)
		if cfg.KASXChainId == "" {
			logger.Crit("KAS x-chain-id should be set", "key", utils.KASServiceChainXChainIdFlag.Name)
		}
	}
	return cfg
}

func MakeFullNode(ctx *cli.Context) *node.Node {
	stack, cfg := makeConfigNode(ctx)
	scfg := makeServiceChainConfig(ctx)
	scfg.DataDir = cfg.Node.DataDir
	scfg.Name = cfg.Node.Name

	if utils.NetworkTypeFlag.Value == SCNNetworkType && scfg.EnabledSubBridge {
		cfg.CN.NoAccountCreation = !ctx.GlobalBool(utils.ServiceChainNewAccountFlag.Name)
		if !cfg.CN.NoAccountCreation {
			logger.Warn("generated accounts can't be synced with the parent chain since account creation is enabled")
		}

		switch scfg.ServiceChainConsensus {
		case "istanbul":
			utils.RegisterCNService(stack, &cfg.CN)
		case "clique":
			logger.Crit("using clique consensus type is not allowed anymore!")
		default:
			logger.Crit("unknown consensus type for the service chain", "consensus", scfg.ServiceChainConsensus)
		}
	} else {
		utils.RegisterCNService(stack, &cfg.CN)
	}
	utils.RegisterService(stack, &scfg)

	dbfg := makeDBSyncerConfig(ctx)
	utils.RegisterDBSyncerService(stack, &dbfg)

	chaindataFetcherConfig := makeChainDataFetcherConfig(ctx)
	utils.RegisterChainDataFetcherService(stack, &chaindataFetcherConfig)

	return stack
}

func dumpConfig(ctx *cli.Context) error {
	_, cfg := makeConfigNode(ctx)
	comment := ""

	if cfg.CN.Genesis != nil {
		cfg.CN.Genesis = nil
		comment += "# Note: this config doesn't contain the genesis block.\n\n"
	}

	out, err := tomlSettings.Marshal(&cfg)
	if err != nil {
		return err
	}
	io.WriteString(os.Stdout, comment)
	os.Stdout.Write(out)
	return nil
}
