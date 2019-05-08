// Copyright 2018 The klaytn Authors
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
	"github.com/ground-x/klaytn/cmd/utils"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/datasync/dbsyncer"
	"github.com/ground-x/klaytn/log"
	"github.com/ground-x/klaytn/node"
	"github.com/ground-x/klaytn/node/cn"
	"github.com/ground-x/klaytn/node/sc"
	"github.com/ground-x/klaytn/params"
	"gopkg.in/urfave/cli.v1"
	"os"
	"reflect"
	"strings"
	"unicode"

	"github.com/naoina/toml"
	"io"
)

var (
	ConfigFileFlag = cli.StringFlag{
		Name:  "config",
		Usage: "TOML configuration file",
	}
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
		CN:   cn.DefaultConfig,
		Node: defaultNodeConfig(),
	}

	// Load config file.
	if file := ctx.GlobalString(ConfigFileFlag.Name); file != "" {
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

func makeDBSyncerConfig(ctx *cli.Context) dbsyncer.DBConfig {
	cfg := dbsyncer.DefaultDBConfig

	if ctx.GlobalBool(utils.EnableDBSyncerFlag.Name) {
		cfg.EnabledDBSyncer = true

		if ctx.GlobalIsSet(utils.DBHostFlag.Name) {
			dbhost := ctx.GlobalString(utils.DBHostFlag.Name)
			cfg.DBHost = dbhost
		}
		if ctx.GlobalIsSet(utils.DBPortFlag.Name) {
			dbports := ctx.GlobalString(utils.DBPortFlag.Name)
			cfg.DBPort = dbports
		}
		if ctx.GlobalIsSet(utils.DBUserFlag.Name) {
			dbuser := ctx.GlobalString(utils.DBUserFlag.Name)
			cfg.DBUser = dbuser
		}
		if ctx.GlobalIsSet(utils.DBPasswordFlag.Name) {
			dbpasswd := ctx.GlobalString(utils.DBPasswordFlag.Name)
			cfg.DBPassword = dbpasswd
		}
		if ctx.GlobalIsSet(utils.DBNameFlag.Name) {
			dbname := ctx.GlobalString(utils.DBNameFlag.Name)
			cfg.DBName = dbname
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
	}

	return *cfg
}

func makeServiceChainConfig(ctx *cli.Context) (config sc.SCConfig) {
	cfg := sc.SCConfig{
		// TODO-Klaytn this value is temp for test
		NetworkId: 1,
		MaxPeer:   50,
	}

	// bridge service
	if ctx.GlobalBool(utils.EnabledBridgeFlag.Name) {
		cfg.EnabledBridge = true

		cfg.BridgePort = fmt.Sprintf(":%d", ctx.GlobalInt(utils.BridgeListenPortFlag.Name))

		if ctx.GlobalBool(utils.IsMainBridgeFlag.Name) {
			cfg.IsMainBridge = true
		} else {
			cfg.IsMainBridge = false

			if ctx.GlobalIsSet(utils.MainChainAccountAddrFlag.Name) {
				tempStr := ctx.GlobalString(utils.MainChainAccountAddrFlag.Name)
				if !common.IsHexAddress(tempStr) {
					logger.Crit("Given chainaddr does not meet hex format.", "chainaddr", tempStr)
				}
				tempAddr := common.StringToAddress(tempStr)
				cfg.MainChainAccountAddr = &tempAddr
				logger.Info("A chain address is registered.", "mainChainAccountAddr", *cfg.MainChainAccountAddr)
			}
			cfg.AnchoringPeriod = ctx.GlobalUint64(utils.AnchoringPeriodFlag.Name)
			cfg.SentChainTxsLimit = ctx.GlobalUint64(utils.SentChainTxsLimit.Name)
			cfg.ParentChainURL = ctx.GlobalString(utils.ParentChainURLFlag.Name)
			cfg.VTRecovery = ctx.GlobalBool(utils.VTRecoveryFlag.Name)
			cfg.VTRecoveryInterval = ctx.GlobalUint64(utils.VTRecoveryIntervalFlag.Name)
			cfg.ServiceChainNewAccount = ctx.GlobalBool(utils.ServiceChainNewAccountFlag.Name)
		}

	} else {
		cfg.EnabledBridge = false
	}

	return cfg
}

func MakeFullNode(ctx *cli.Context) *node.Node {
	stack, cfg := makeConfigNode(ctx)
	scfg := makeServiceChainConfig(ctx)
	scfg.DataDir = cfg.Node.DataDir
	scfg.Name = cfg.Node.Name

	if utils.NetworkTypeFlag.Value == "scn" {
		utils.RegisterServiceChainService(stack, &cfg.CN, &scfg)
	} else {
		utils.RegisterCNService(stack, &cfg.CN)
	}
	utils.RegisterService(stack, &scfg)

	dbfg := makeDBSyncerConfig(ctx)
	utils.RegisterDBSyncerService(stack, &dbfg)

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
