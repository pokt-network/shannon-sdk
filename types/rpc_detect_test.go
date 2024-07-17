package types_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/pokt-network/poktroll/proto/types/shared"
	"github.com/stretchr/testify/require"

	"github.com/pokt-network/shannon-sdk/types"
)

var (
	restContentBz           = []byte(`{"key":"value"}`)
	jsonRPCContentBz        = []byte(`{"jsonrpc":"2.0","method":"m","params":[],"id":1}`)
	method                  = "POST"
	requestUrl              = "http://localhost:8080"
	errDefault              = errors.New("error")
	defaultJSONRPCErrorCode = -32000
)

func TestRPCType_DetectRPC(t *testing.T) {
	tests := []struct {
		desc            string
		inputRequest    *types.POKTHTTPRequest
		expectedRPCType shared.RPCType
	}{
		{
			desc: "Detect JSON-RPC",
			inputRequest: &types.POKTHTTPRequest{
				Header: map[string]*types.Header{
					contentTypeHeaderKey: {
						Key:    contentTypeHeaderKey,
						Values: []string{contentTypeHeaderValueJSON},
					},
				},
				Method: method,
				Url:    requestUrl,
				BodyBz: jsonRPCContentBz,
			},
			expectedRPCType: shared.RPCType_JSON_RPC,
		},
		{
			desc: "Detect REST",
			inputRequest: &types.POKTHTTPRequest{
				Header: map[string]*types.Header{
					contentTypeHeaderKey: {
						Key:    contentTypeHeaderKey,
						Values: []string{contentTypeHeaderValueJSON},
					},
				},
				Method: method,
				Url:    requestUrl,
				BodyBz: restContentBz,
			},
			expectedRPCType: shared.RPCType_REST,
		},
		{
			desc: "Unknown RPC",
			inputRequest: &types.POKTHTTPRequest{
				Header: map[string]*types.Header{
					contentTypeHeaderKey: {
						Key:    contentTypeHeaderKey,
						Values: []string{contentTypeHeaderValueJSON},
					},
				},
				Method: method,
				Url:    "",
				BodyBz: restContentBz,
			},
			expectedRPCType: shared.RPCType_UNKNOWN_RPC,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			actualRPCType := tt.inputRequest.GetRPCType()

			require.Equal(t, tt.expectedRPCType, actualRPCType)
		})
	}
}

func TestRPCType_FormatError(t *testing.T) {
	tests := []struct {
		desc                  string
		inputError            error
		isInternal            bool
		inputRequest          *types.POKTHTTPRequest
		expectedErrorResponse *types.POKTHTTPResponse
	}{
		{
			desc:       "Format JSON-RPC error",
			inputError: errDefault,
			isInternal: false,
			inputRequest: &types.POKTHTTPRequest{
				Header: map[string]*types.Header{
					contentTypeHeaderKey: {
						Key:    contentTypeHeaderKey,
						Values: []string{contentTypeHeaderValueJSON},
					},
				},
				Method: method,
				Url:    requestUrl,
				BodyBz: jsonRPCContentBz,
			},
			expectedErrorResponse: &types.POKTHTTPResponse{
				StatusCode: http.StatusOK,
				Header: map[string]*types.Header{
					contentTypeHeaderKey: {
						Key:    contentTypeHeaderKey,
						Values: []string{contentTypeHeaderValueJSON},
					},
				},
				BodyBz: []byte(fmt.Sprintf(
					`{"error":{"code":%d,"data":null,"message":"%s"},"id":1,"jsonrpc":"2.0"}`,
					defaultJSONRPCErrorCode,
					errDefault.Error(),
				)),
			},
		},
		{
			desc:       "Format REST error",
			inputError: errDefault,
			isInternal: false,
			inputRequest: &types.POKTHTTPRequest{
				Header: map[string]*types.Header{
					contentTypeHeaderKey: {
						Key:    contentTypeHeaderKey,
						Values: []string{"text/plain"},
					},
				},
				Method: method,
				Url:    requestUrl,
				BodyBz: restContentBz,
			},
			expectedErrorResponse: &types.POKTHTTPResponse{
				StatusCode: http.StatusBadRequest,
				Header: map[string]*types.Header{
					contentTypeHeaderKey: {
						Key:    contentTypeHeaderKey,
						Values: []string{"text/plain"},
					},
				},
				BodyBz: []byte(errDefault.Error()),
			},
		},
		{
			desc:       "Format unsupported RPC type error",
			inputError: errDefault,
			isInternal: false,
			inputRequest: &types.POKTHTTPRequest{
				Header: map[string]*types.Header{
					contentTypeHeaderKey: {
						Key:    contentTypeHeaderKey,
						Values: []string{contentTypeHeaderValueJSON},
					},
				},
				Method: method,
				Url:    "",
				BodyBz: restContentBz,
			},
			expectedErrorResponse: &types.POKTHTTPResponse{
				StatusCode: http.StatusBadRequest,
				Header: map[string]*types.Header{
					contentTypeHeaderKey: {
						Key:    contentTypeHeaderKey,
						Values: []string{"text/plain"},
					},
				},
				BodyBz: []byte(`unsupported rpc type`),
			},
		},
		{
			desc:       "Format internal JSON-RPC error",
			inputError: errDefault,
			isInternal: true,
			inputRequest: &types.POKTHTTPRequest{
				Header: map[string]*types.Header{
					contentTypeHeaderKey: {
						Key:    contentTypeHeaderKey,
						Values: []string{contentTypeHeaderValueJSON},
					},
				},
				Method: method,
				Url:    requestUrl,
				BodyBz: jsonRPCContentBz,
			},
			expectedErrorResponse: &types.POKTHTTPResponse{
				StatusCode: http.StatusOK,
				Header: map[string]*types.Header{
					contentTypeHeaderKey: {
						Key:    contentTypeHeaderKey,
						Values: []string{contentTypeHeaderValueJSON},
					},
				},
				BodyBz: []byte(fmt.Sprintf(
					`{"error":{"code":%d,"data":null,"message":"%s"},"id":1,"jsonrpc":"2.0"}`,
					defaultJSONRPCErrorCode,
					"Internal error",
				)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			actualErrorResponse, _ := tt.inputRequest.FormatError(tt.inputError, tt.isInternal)

			require.Equal(t, tt.expectedErrorResponse.StatusCode, actualErrorResponse.StatusCode)
			require.Equal(t, tt.expectedErrorResponse.BodyBz, actualErrorResponse.BodyBz)

			for key, header := range tt.expectedErrorResponse.Header {
				for i, value := range header.Values {
					require.Equal(t, value, actualErrorResponse.Header[key].Values[i])
				}
			}
		})
	}
}
