// Copyright 2018 The klaytn Authors
// Copyright 2016 The go-ethereum Authors
// This file is part of go-ethereum.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.
//
// This file is derived from node/defaults.go (2018/06/04).
// Modified and improved for the klaytn development.

package node

import (
	"github.com/ground-x/klaytn/networks/p2p"
	"github.com/ground-x/klaytn/networks/p2p/nat"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	DefaultHTTPHost = "localhost" // Default host interface for the HTTP RPC server
	DefaultHTTPPort = 8545        // Default TCP port for the HTTP RPC server
	DefaultWSHost   = "localhost" // Default host interface for the websocket RPC server
	DefaultWSPort   = 8546        // Default TCP port for the websocket RPC server
	DefaultGRPCHost = "localhost" // Default host interface for the gRPC server
	DefaultGRPCPort = 8547        // Default TCP port for the gRPC server
)

// DefaultConfig contains reasonable default settings.
var DefaultConfig = Config{
	DBType:           DefaultDBType(),
	DataDir:          DefaultDataDir(),
	HTTPPort:         DefaultHTTPPort,
	HTTPModules:      []string{"net", "web3"},
	HTTPVirtualHosts: []string{"localhost"},
	WSPort:           DefaultWSPort,
	WSModules:        []string{"net", "web3"},
	GRPCPort:         DefaultGRPCPort,
	P2P: p2p.Config{
		ListenAddr: ":30303",
		MaxPeers:   25,
		NAT:        nat.Any(),
	},
}

func DefaultDBType() string {
	return "leveldb"
}

// DefaultDataDir is the default data directory to use for the databases and other
// persistence requirements.
func DefaultDataDir() string {
	// Try to place the data folder in the user's home dir
	dirname := filepath.Base(os.Args[0])
	if dirname == "" {
		dirname = "klay"
	}
	home := homeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", strings.ToUpper(dirname))
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", strings.ToUpper(dirname))
		} else {
			return filepath.Join(home, "."+dirname)
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}
