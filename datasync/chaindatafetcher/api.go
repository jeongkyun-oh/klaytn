// Copyright 2020 The klaytn Authors
// This file is part of the klaytn library.
//
// The klaytn library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The klaytn library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the klaytn library. If not, see <http://www.gnu.org/licenses/>.

package chaindatafetcher

type PublicChainDataFetcherAPI struct {
	f *ChainDataFetcher
}

func NewPublicChainDataFetcherAPI(f *ChainDataFetcher) *PublicChainDataFetcherAPI {
	return &PublicChainDataFetcherAPI{f: f}
}

func (api *PublicChainDataFetcherAPI) Start() error {
	return api.f.start()
}

func (api *PublicChainDataFetcherAPI) Stop() error {
	return api.f.stop()
}

func (api *PublicChainDataFetcherAPI) StartWithRange(start, end uint64, reqType uint) error {
	return api.f.startRange(start, end, requestType(reqType))
}

func (api *PublicChainDataFetcherAPI) StopWithRange() error {
	return api.f.stopRange()
}

func (api *PublicChainDataFetcherAPI) Status() {
	// TODO-ChainDataFetcher update status
}

func (api *PublicChainDataFetcherAPI) ReadCheckpoint() (int64, error) {
	return api.f.repo.ReadCheckpoint()
}

func (api *PublicChainDataFetcherAPI) WriteCheckpoint(checkpoint int64) error {
	return api.f.repo.WriteCheckpoint(checkpoint)
}

// GetConfig returns the configuration setting of the launched chaindata fetcher.
func (api *PublicChainDataFetcherAPI) GetConfig() *ChainDataFetcherConfig {
	return api.f.config
}