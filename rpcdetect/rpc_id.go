package rpcdetect

import (
	"bytes"
	"io"
	"net/http"

	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
)

var unsupportedRPCType = []byte(`unsupported rpc type`)

func GetRPCType(request *http.Request) (rpcType sharedtypes.RPCType, err error) {
	if isJSONRPC(request) {
		return sharedtypes.RPCType_JSON_RPC, nil
	}
	if isREST(request) {
		return sharedtypes.RPCType_REST, nil
	}

	return sharedtypes.RPCType_UNKNOWN_RPC, nil
}

func FormatError(
	err error,
	request *http.Request,
	rpcType sharedtypes.RPCType,
	isInternal bool,
) *http.Response {
	switch rpcType {
	case sharedtypes.RPCType_JSON_RPC:
		return formatJSONRPCError(request, err, isInternal)
	case sharedtypes.RPCType_REST:
		return formatRESTError(request, err, isInternal)
	default:
		return formatGenericHTTPError()
	}
}

func formatGenericHTTPError() *http.Response {
	header := http.Header{}
	header.Set("Content-Type", "text/plain")
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(unsupportedRPCType)),
	}
}
