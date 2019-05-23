// Copyright 2018 The klaytn Authors
// Copyright 2016 The go-ethereum Authors
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
// This file is derived from cmd/geth/main.go (2018/06/04).
// Modified and improved for the klaytn development.

package nodecmd

import (
	"fmt"
	"github.com/ground-x/klaytn/accounts"
	"github.com/ground-x/klaytn/accounts/keystore"
	"github.com/ground-x/klaytn/api/debug"
	"github.com/ground-x/klaytn/client"
	"github.com/ground-x/klaytn/cmd/utils"
	"github.com/ground-x/klaytn/log"
	"github.com/ground-x/klaytn/node"
	"github.com/ground-x/klaytn/node/cn"
	"gopkg.in/urfave/cli.v1"
	"os"
	"strings"
)

const (
	clientIdentifier = "klay" // Client identifier to advertise over the network
	SCNNetworkType   = "scn"  // Service Chain Network
	MNNetworkType    = "mn"   // Mainnet Network
)

// runKlaytnNode is the main entry point into the system if no special subcommand is ran.
// It creates a default node based on the command line arguments and runs it in
// blocking mode, waiting for it to be shut down.
func RunKlaytnNode(ctx *cli.Context) error {
	fullNode := MakeFullNode(ctx)
	startNode(ctx, fullNode)
	fullNode.Wait()
	return nil
}

// startNode boots up the system node and all registered protocols, after which
// it unlocks any requested accounts, and starts the RPC/IPC interfaces and the
// miner.
func startNode(ctx *cli.Context, stack *node.Node) {
	debug.Memsize.Add("node", stack)

	// Start up the node itself
	utils.StartNode(stack)

	// Unlock any account specifically requested
	ks := stack.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)

	passwords := utils.MakePasswordList(ctx)
	unlocks := strings.Split(ctx.GlobalString(utils.UnlockedAccountFlag.Name), ",")
	for i, account := range unlocks {
		if trimmed := strings.TrimSpace(account); trimmed != "" {
			UnlockAccount(ctx, ks, trimmed, i, passwords)
		}
	}
	// Register wallet event handlers to open and auto-derive wallets
	events := make(chan accounts.WalletEvent, 16)
	stack.AccountManager().Subscribe(events)

	go func() {
		// Create a chain state reader for self-derivation
		rpcClient, err := stack.Attach()
		if err != nil {
			log.Fatalf("Failed to attach to self: %v", err)
		}
		stateReader := client.NewClient(rpcClient)

		// Open any wallets already attached
		for _, wallet := range stack.AccountManager().Wallets() {
			if err := wallet.Open(""); err != nil {
				logger.Error("Failed to open wallet", "url", wallet.URL(), "err", err)
			}
		}
		// Listen for wallet event till termination
		for event := range events {
			switch event.Kind {
			case accounts.WalletArrived:
				if err := event.Wallet.Open(""); err != nil {
					logger.Error("New wallet appeared, failed to open", "url", event.Wallet.URL(), "err", err)
				}
			case accounts.WalletOpened:
				status, _ := event.Wallet.Status()
				logger.Info("New wallet appeared", "url", event.Wallet.URL(), "status", status)

				if event.Wallet.URL().Scheme == "ledger" {
					event.Wallet.SelfDerive(accounts.DefaultLedgerBaseDerivationPath, stateReader)
				} else {
					event.Wallet.SelfDerive(accounts.DefaultBaseDerivationPath, stateReader)
				}

			case accounts.WalletDropped:
				logger.Info("Old wallet dropped", "url", event.Wallet.URL())
				event.Wallet.Close()
			}
		}
	}()

	if utils.NetworkTypeFlag.Value == SCNNetworkType {
		startServiceChainService(ctx, stack)
	} else {
		startKlaytnAuxiliaryService(ctx, stack)
	}
}

func startKlaytnAuxiliaryService(ctx *cli.Context, stack *node.Node) {
	var cn *cn.CN
	if err := stack.Service(&cn); err != nil {
		log.Fatalf("Klaytn service not running: %v", err)
	}

	// TODO-Klaytn-NodeCmd disable accept tx before finishing sync.
	if err := cn.StartMining(false); err != nil {
		log.Fatalf("Failed to start mining: %v", err)
	}
}

func startServiceChainService(ctx *cli.Context, stack *node.Node) {
	var scn *cn.ServiceChain
	if err := stack.Service(&scn); err != nil {
		log.Fatalf("Klaytn service not running: %v", err)
	}

	// TODO-Klaytn-NodeCmd disable accept tx before finishing sync.
	if err := scn.StartMining(false); err != nil {
		log.Fatalf("Failed to start mining: %v", err)
	}
}

func CommandNotExist(context *cli.Context, s string) {
	cli.ShowAppHelp(context)
	fmt.Printf("Error: Unknown command \"%s\"\n", s)
	os.Exit(1)
}
