package rpcdetect

import (
	"net/http"

	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
	"google.golang.org/protobuf/proto"

	"github.com/pokt-network/shannon-sdk/httpcodec"
)

var (
	contentTypeHeaderKey           = "Content-Type"
	unsupportedRPCTypeErrorReply   *httpcodec.HTTPResponse
	unsupportedRPCTypeErrorReplyBz []byte
)

func init() {
	unsupportedRPCTypeErrorReply = &httpcodec.HTTPResponse{
		StatusCode: http.StatusBadGateway,
		Header:     map[string]string{contentTypeHeaderKey: "text/plain"},
		Body:       []byte(`unsupported rpc type`),
	}

	var err error
	unsupportedRPCTypeErrorReplyBz, err = proto.Marshal(unsupportedRPCTypeErrorReply)
	if err != nil {
		panic(err)
	}
}

func GetRPCType(poktRequest *httpcodec.HTTPRequest) sharedtypes.RPCType {
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
	request *httpcodec.HTTPRequest,
	rpcType sharedtypes.RPCType,
	isInternal bool,
) (*httpcodec.HTTPResponse, []byte) {
	switch rpcType {
	case sharedtypes.RPCType_JSON_RPC:
		return formatJSONRPCError(err, request, isInternal)
	case sharedtypes.RPCType_REST:
		return formatRESTError(err, request, isInternal)
	default:
		return unsupportedRPCTypeErrorReply, unsupportedRPCTypeErrorReplyBz
	}
}
