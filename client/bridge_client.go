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
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with go-ethereum library. If not, see <http://www.gnu.org/licenses/>.
//
// This file is derived from ethclient/ethclient.go (2018/06/04).
// Modified and improved for the klaytn development.

package client

import (
	"context"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/networks/p2p"
)

// BridgeAddPeerOnParentChain can add a static peer on bridge node for service chain.
func (ec *Client) BridgeAddPeerOnBridge(ctx context.Context, url string) (bool, error) {
	var result bool
	err := ec.c.CallContext(ctx, &result, "bridge_addPeer", url)
	return result, err
}

// BridgeRemovePeerOnParentChain can remove a static peer on bridge node.
func (ec *Client) BridgeRemovePeerOnBridge(ctx context.Context, url string) (bool, error) {
	var result bool
	err := ec.c.CallContext(ctx, &result, "bridge_removePeer", url)
	return result, err
}

// BridgePeersOnBridge returns the peer list of bridge node for service chain.
func (ec *Client) BridgePeersOnBridge(ctx context.Context) ([]*p2p.PeerInfo, error) {
	var result []*p2p.PeerInfo
	err := ec.c.CallContext(ctx, &result, "bridge_peers")
	return result, err
}

// BridgeNodeInfo returns the node information
func (ec *Client) BridgeNodeInfo(ctx context.Context) (*p2p.NodeInfo, error) {
	var result p2p.NodeInfo
	err := ec.c.CallContext(ctx, &result, "bridge_nodeInfo")
	return &result, err
}

// BridgeGetChildChainIndexingEnabled can get if child chain indexing is enabled or not.
func (ec *Client) BridgeGetChildChainIndexingEnabled(ctx context.Context) (bool, error) {
	var result bool
	err := ec.c.CallContext(ctx, &result, "bridge_getChildChainIndexingEnabled")
	return result, err
}

// BridgeConvertChildChainBlockHashToParentChainTxHash can convert child chain block hash to
// anchoring tx hash which contain anchored data.
func (ec *Client) BridgeConvertChildChainBlockHashToParentChainTxHash(ctx context.Context, ccBlockHash common.Hash) (common.Hash, error) {
	var txHash common.Hash
	err := ec.c.CallContext(ctx, &txHash, "bridge_convertChildChainBlockHashToParentChainTxHash", ccBlockHash)
	return txHash, err
}

// BridgeGetReceiptFromParentChain can get the receipt of child chain tx from parent node.
func (ec *Client) BridgeGetReceiptFromParentChain(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	var result types.Receipt
	err := ec.c.CallContext(ctx, &result, "bridge_getReceiptFromParentChain", hash)
	return &result, err
}

// BridgeGetChainAccountAddr can get the chain address to sign chain transaction in service chain.
func (ec *Client) GetChainAccountAddr(ctx context.Context) (common.Address, error) {
	var result common.Address
	err := ec.c.CallContext(ctx, &result, "bridge_getChainAccountAddr")
	return result, err
}

// BridgeGetLatestAnchoredBlockNumber can return the latest anchored block number.
func (ec *Client) BridgeGetLatestAnchoredBlockNumber(ctx context.Context) (uint64, error) {
	var result uint64
	err := ec.c.CallContext(ctx, &result, "bridge_getLatestAnchoredBlockNumber")
	return result, err
}

// BridgeEnableAnchoring can enable anchoring function and return the set value.
func (ec *Client) BridgeEnableAnchoring(ctx context.Context) (bool, error) {
	return ec.setAnchoring(ctx, true)
}

// BridgeDisableAnchoring can disable anchoring function and return the set value.
func (ec *Client) BridgeDisableAnchoring(ctx context.Context) (bool, error) {
	return ec.setAnchoring(ctx, false)
}

// setAnchoring can set if anchoring is enabled or not.
func (ec *Client) setAnchoring(ctx context.Context, enable bool) (bool, error) {
	var result bool
	err := ec.c.CallContext(ctx, &result, "bridge_anchoring", enable)
	return result, err
}

// BridgeGetAnchoringPeriod can get the block period to anchor chain data.
func (ec *Client) BridgeGetAnchoringPeriod(ctx context.Context) (uint64, error) {
	var result uint64
	err := ec.c.CallContext(ctx, &result, "bridge_getAnchoringPeriod")
	return result, err
}

// BridgeGetSentChainTxsLimit can get the maximum number of transaction which child peer can send to parent peer once.
func (ec *Client) BridgeGetSentChainTxsLimit(ctx context.Context) (uint64, error) {
	var result uint64
	err := ec.c.CallContext(ctx, &result, "bridge_getSentChainTxsLimit")
	return result, err
}