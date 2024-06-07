package rpcdetect

import (
	"encoding/json"
	"net/http"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/pokt-network/shannon-sdk/types"
)

var (
	defaultJSONRPCErrorReply   *types.POKTHTTPResponse
	defaultJSONRPCErrorReplyBz []byte
)

func init() {
	// Initialize the default JSON-RPC error reply
	header := &types.Header{
		Key:    contentTypeHeaderKey,
		Values: []string{"application/json"},
	}
	headers := map[string]*types.Header{contentTypeHeaderKey: header}

	defaultJSONRPCErrorReply = &types.POKTHTTPResponse{
		StatusCode: http.StatusInternalServerError,
		Header:     headers,
		BodyBz:     []byte(`{"jsonrpc":"2.0","id":0,"error":{"code":-32000,"message":"Internal error","data":null}}`),
	}

	var err error
	defaultJSONRPCErrorReplyBz, err = proto.Marshal(defaultJSONRPCErrorReply)
	if err != nil {
		panic(err)
	}
}

type jsonRPCPayload struct {
	Id      uint64 `json:"id"`
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
}

// isJSONRPC checks if the given POKTHTTPRequest is a JSON-RPC request.
func isJSONRPC(poktRequest *types.POKTHTTPRequest) bool {
	contentType, ok := poktRequest.Header[contentTypeHeaderKey]
	if !ok {
		return false
	}

	if !slices.Contains(contentType.Values, "application/json") {
		return false
	}

	payload, err := readJSONRPCPayload(poktRequest.BodyBz)
	if err != nil {
		return false
	}

	if payload.Id == 0 || len(payload.JSONRPC) == 0 || len(payload.Method) == 0 {
		return false
	}

	return true
}

// formatJSONRPCError formats the given error into a JSON-RPC error response.
func formatJSONRPCError(
	err error,
	poktRequestBz *types.POKTHTTPRequest,
	isInternal bool,
) (*types.POKTHTTPResponse, []byte) {
	errorMsg := err.Error()
	if isInternal {
		errorMsg = "Internal error"
	}

	requestId := uint64(0)
	payload, err := readJSONRPCPayload(poktRequestBz.BodyBz)
	if err == nil {
		requestId = payload.Id
	}

	errorReplyPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestId,
		"error": map[string]interface{}{
			"code":    -32000,
			"message": errorMsg,
			"data":    nil,
		},
	}

	responseBodyBz, err := json.Marshal(errorReplyPayload)
	if err != nil {
		return defaultJSONRPCErrorReply, defaultJSONRPCErrorReplyBz
	}

	header := &types.Header{
		Key:    contentTypeHeaderKey,
		Values: []string{"application/json"},
	}
	headers := map[string]*types.Header{contentTypeHeaderKey: header}
	poktResponse := &types.POKTHTTPResponse{
		StatusCode: http.StatusOK,
		Header:     headers,
		BodyBz:     responseBodyBz,
	}

	responseBz, err := proto.Marshal(poktResponse)
	if err != nil {
		return defaultJSONRPCErrorReply, defaultJSONRPCErrorReplyBz
	}

	return poktResponse, responseBz
}

// readJSONRPCPayload reads and parses the JSON-RPC payload from the given request body.
func readJSONRPCPayload(requestBodyBz []byte) (*jsonRPCPayload, error) {
	var payload jsonRPCPayload
	if err := json.Unmarshal(requestBodyBz, &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}
