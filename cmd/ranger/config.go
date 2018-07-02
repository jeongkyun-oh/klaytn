package main

import (
	"github.com/naoina/toml"
	"reflect"
	"unicode"
	"fmt"
	"os"
	"bufio"
	"io"
	"gopkg.in/urfave/cli.v1"
	"github.com/ground-x/go-gxplatform/cmd/utils"
	"github.com/ground-x/go-gxplatform/node"
	rnpkg "github.com/ground-x/go-gxplatform/ranger"
	"errors"
	"github.com/ground-x/go-gxplatform/params"
)

var (
	dumpConfigCommand = cli.Command{
		Action:      utils.MigrateFlags(dumpConfig),
		Name:        "dumpconfig",
		Usage:       "Show configuration values",
		ArgsUsage:   "",
		Flags:       append(append(nodeFlags, rpcFlags...)),
		Category:    "MISCELLANEOUS COMMANDS",
		Description: `The dumpconfig command shows configuration values.`,
	}

	configFileFlag = cli.StringFlag{
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

type rangerConfig struct {
	Gxp  rnpkg.Config
	Node node.Config
}

func loadConfig(file string, cfg *rangerConfig) error {
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

func defaultRangerConfig() node.Config {
	cfg := node.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = params.VersionWithCommit(gitCommit)
	cfg.HTTPModules = append(cfg.HTTPModules, "gxp")
	cfg.WSModules = append(cfg.WSModules, "gxp")
	cfg.IPCPath = "gxp.ipc"
	return cfg
}

func makeConfigRanger(ctx *cli.Context) (*node.Node, rangerConfig) {
	// Load defaults.
	cfg := rangerConfig{
		Gxp:  rnpkg.DefaultConfig ,
		Node: defaultRangerConfig(),
	}

	// Load config file.
	if file := ctx.GlobalString(configFileFlag.Name); file != "" {
		if err := loadConfig(file, &cfg); err != nil {
			utils.Fatalf("%v", err)
		}
	}

	// Apply flags.
	utils.SetNodeConfig(ctx, &cfg.Node)
	stack, err := node.New(&cfg.Node)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}

	return stack, cfg
}

func makeRanger(ctx *cli.Context) *node.Node {
	stack, cfg := makeConfigRanger(ctx)

	utils.RegisterRanagerService(stack, &cfg.Gxp)

	return stack
}

func dumpConfig(ctx *cli.Context) error {
	_, cfg := makeConfigRanger(ctx)
	comment := ""

	if cfg.Gxp.Genesis != nil {
		cfg.Gxp.Genesis = nil
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
