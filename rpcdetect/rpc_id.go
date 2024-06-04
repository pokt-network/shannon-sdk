package rpcdetect

import (
	"net/http"

	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
	"google.golang.org/protobuf/proto"

	"github.com/pokt-network/shannon-sdk/types"
)

var (
	contentTypeHeaderKey           = "Content-Type"
	unsupportedRPCTypeErrorReply   *types.POKTHTTPResponse
	unsupportedRPCTypeErrorReplyBz []byte
)

func init() {
	header := &types.Header{
		Key:    contentTypeHeaderKey,
		Values: []string{"text/plain"},
	}
	headers := map[string]*types.Header{contentTypeHeaderKey: header}

	unsupportedRPCTypeErrorReply = &types.POKTHTTPResponse{
		StatusCode: http.StatusBadGateway,
		Header:     headers,
		BodyBz:     []byte(`unsupported rpc type`),
	}

	var err error
	unsupportedRPCTypeErrorReplyBz, err = proto.Marshal(unsupportedRPCTypeErrorReply)
	if err != nil {
		panic(err)
	}
}

func GetRPCType(poktRequest *types.POKTHTTPRequest) sharedtypes.RPCType {
	if isJSONRPC(poktRequest) {
		return sharedtypes.RPCType_JSON_RPC
	}
	if isREST(poktRequest) {
		return sharedtypes.RPCType_REST
	}

	return sharedtypes.RPCType_UNKNOWN_RPC
}

func FormatError(
	err error,
	request *types.POKTHTTPRequest,
	rpcType sharedtypes.RPCType,
	isInternal bool,
) (*types.POKTHTTPResponse, []byte) {
	switch rpcType {
	case sharedtypes.RPCType_JSON_RPC:
		return formatJSONRPCError(err, request, isInternal)
	case sharedtypes.RPCType_REST:
		return formatRESTError(err, request, isInternal)
	default:
		return unsupportedRPCTypeErrorReply, unsupportedRPCTypeErrorReplyBz
	}
}
