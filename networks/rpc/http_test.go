// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
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

package rpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/klaytn/klaytn/common"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

func TestHTTPErrorResponseWithDelete(t *testing.T) {
	testHTTPErrorResponse(t, http.MethodDelete, contentType, "", http.StatusMethodNotAllowed)
}

func TestHTTPErrorResponseWithPut(t *testing.T) {
	testHTTPErrorResponse(t, http.MethodPut, contentType, "", http.StatusMethodNotAllowed)
}

func TestHTTPErrorResponseWithMaxContentLength(t *testing.T) {
	body := make([]rune, common.MaxRequestContentLength+1)
	testHTTPErrorResponse(t,
		http.MethodPost, contentType, string(body), http.StatusRequestEntityTooLarge)
}

func TestHTTPErrorResponseWithEmptyContentType(t *testing.T) {
	testHTTPErrorResponse(t, http.MethodPost, "", "", http.StatusUnsupportedMediaType)
}

func TestHTTPErrorResponseWithValidRequest(t *testing.T) {
	testHTTPErrorResponse(t, http.MethodPost, contentType, "", 0)
}

func testHTTPErrorResponse(t *testing.T, method, contentType, body string, expected int) {
	request := httptest.NewRequest(method, "http://url.com", strings.NewReader(body))
	request.Header.Set("content-type", contentType)
	if code, _ := validateRequest(request); code != expected {
		t.Fatalf("response code should be %d not %d", expected, code)
	}
}

type testServer struct {
	srvType     string
	httpSrv     *http.Server
	fasthttpSrv *fasthttp.Server
}

func (srv *testServer) serve(l net.Listener) error {
	switch srv.srvType {
	case "http":
		return srv.httpSrv.Serve(l)
	case "fasthttp":
		return srv.fasthttpSrv.Serve(l)
	}
	return errors.New("no supported server type")
}

func (srv *testServer) shutdown() error {
	switch srv.srvType {
	case "http":
		return srv.httpSrv.Shutdown(context.Background())
	case "fasthttp":
		return srv.fasthttpSrv.Shutdown()
	}
	return errors.New("no supported server type")
}

func createTimeoutTestServer(srvType string, operationTime time.Duration, timeouts HTTPTimeouts) (*testServer, error) {
	h := http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		time.Sleep(operationTime)
	})

	switch srvType {
	case "http", "ws":
		return &testServer{
			srvType: srvType,
			httpSrv: &http.Server{
				Handler:      h,
				ReadTimeout:  timeouts.ReadTimeout,
				WriteTimeout: timeouts.WriteTimeout,
				IdleTimeout:  timeouts.IdleTimeout,
			},
		}, nil
	case "fasthttp", "fastws":
		return &testServer{
			srvType: srvType,
			fasthttpSrv: &fasthttp.Server{
				Handler:      fasthttpadaptor.NewFastHTTPHandler(h),
				ReadTimeout:  timeouts.ReadTimeout,
				WriteTimeout: timeouts.WriteTimeout,
				IdleTimeout:  timeouts.IdleTimeout,
			},
		}, nil
	}

	return nil, errors.New("no supported server type")
}

func testGetRequst(addr string) error {
	_, err := http.DefaultClient.Get("http://" + addr)
	if err != nil {
		fmt.Println(err)
		return err
	}
	return err
}

func testTimeoutServer(t *testing.T, expected string, srv *testServer) {
	listener, _ := net.Listen("tcp", "localhost:0")
	addr := listener.Addr().String()

	go srv.serve(listener)
	defer srv.shutdown()

	err := testGetRequst(addr)
	if expected == "" {
		assert.Nil(t, err)
	} else {
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), expected))
	}
}

func TestHTTPTimeout(t *testing.T) {
	operationTime := 200 * time.Millisecond
	type testcase struct {
		srvType  string
		timeouts HTTPTimeouts
		expected string
	}

	testcases := []testcase{
		{"http", HTTPTimeouts{
			ReadTimeout:  500 * time.Millisecond,
			WriteTimeout: 500 * time.Millisecond,
			IdleTimeout:  500 * time.Millisecond},
			"",
		},
		{"http", HTTPTimeouts{
			ReadTimeout:  500 * time.Millisecond,
			WriteTimeout: 100 * time.Millisecond,
			IdleTimeout:  500 * time.Millisecond},
			"EOF",
		},
		// TODO-Klaytn: write test codes for fasthttp, ws, fastws
	}

	for _, tc := range testcases {
		srv, err := createTimeoutTestServer(tc.srvType, operationTime, tc.timeouts)
		assert.NoError(t, err)
		testTimeoutServer(t, tc.expected, srv)
	}
}
