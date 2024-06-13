package types

import (
	"net/http"

	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
	"google.golang.org/protobuf/proto"
)

const (
	contentTypeHeaderKey       = "Content-Type"
	contentTypeHeaderValueJSON = "application/json"
	contentTypeHeaderValueText = "text/plain"
	defaultErrorMessage        = "Internal error"
)

var (
	unsupportedRPCTypeErrorReply   *POKTHTTPResponse
	unsupportedRPCTypeErrorReplyBz []byte
)

// init initializes the package level variables such as the unsupported RPC type error reply.
func init() {
	// Initialize the default unsupported RPC type error reply and panic if it fails.
	// This is done to make the program exit early if the default unsupported RPC type
	// error reply fails to be marshaled.
	initDefaultUnsupportedRPCTypeErrorReply()
}

// GetRPCType returns the RPC type of a POKTHTTPRequest.
func (poktRequest *POKTHTTPRequest) GetRPCType() sharedtypes.RPCType {
	if poktRequest.isJSONRPC() {
		return sharedtypes.RPCType_JSON_RPC
	}
	if poktRequest.isREST() {
		return sharedtypes.RPCType_REST
	}

	return sharedtypes.RPCType_UNKNOWN_RPC
}

// FormatError formats the given error into a POKTHTTPResponse and its
// corresponding byte representation.
func (request *POKTHTTPRequest) FormatError(
	err error,
	isInternal bool,
) (*POKTHTTPResponse, []byte) {
	rpcType := request.GetRPCType()

	switch rpcType {
	case sharedtypes.RPCType_JSON_RPC:
		return request.formatJSONRPCError(err, isInternal)
	case sharedtypes.RPCType_REST:
		return request.formatRESTError(err, isInternal)
	default:
		return unsupportedRPCTypeErrorReply, unsupportedRPCTypeErrorReplyBz
	}
}

// initDefaultUnsupportedRPCTypeErrorReply initializes the unsupported RPC type error reply.
// This function is called before the main function and panics if it fails to marshal
// the unsupported RPC type error reply, making the program exit early.
func initDefaultUnsupportedRPCTypeErrorReply() {
	header := &Header{
		Key:    contentTypeHeaderKey,
		Values: []string{contentTypeHeaderValueText},
	}
	headers := map[string]*Header{contentTypeHeaderKey: header}

	unsupportedRPCTypeErrorReply = &POKTHTTPResponse{
		StatusCode: http.StatusBadRequest,
		Header:     headers,
		BodyBz:     []byte(`unsupported rpc type`),
	}

	var err error
	unsupportedRPCTypeErrorReplyBz, err = proto.Marshal(unsupportedRPCTypeErrorReply)
	if err != nil {
		panic(err)
	}
}
