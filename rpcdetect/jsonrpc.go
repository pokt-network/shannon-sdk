package rpcdetect

import (
	"encoding/json"
	"net/http"

	"google.golang.org/protobuf/proto"

	"github.com/pokt-network/shannon-sdk/httpcodec"
)

var (
	defaultJSONRPCErrorReply   *httpcodec.HTTPResponse
	defaultJSONRPCErrorReplyBz []byte
)

func init() {
	defaultJSONRPCErrorReply = &httpcodec.HTTPResponse{
		StatusCode: http.StatusInternalServerError,
		Header:     map[string]string{contentTypeHeaderKey: "application/json"},
		Body:       []byte(`{"jsonrpc":"2.0","id":0,"error":{"code":-32000,"message":"Internal error","data":null}}`),
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

func isJSONRPC(poktRequest *httpcodec.HTTPRequest) bool {
	if poktRequest.Header[contentTypeHeaderKey] != "application/json" {
		return false
	}

	payload, err := readJSONRPCPayload(poktRequest.Body)
	if err != nil {
		return false
	}

	if payload.Id == 0 || len(payload.JSONRPC) == 0 || len(payload.Method) == 0 {
		return false
	}

	return true
}

func formatJSONRPCError(
	err error,
	poktRequestBz *httpcodec.HTTPRequest,
	isInternal bool,
) (*httpcodec.HTTPResponse, []byte) {
	errorMsg := err.Error()
	statusCode := http.StatusBadRequest
	if isInternal {
		errorMsg = "Internal error"
		statusCode = http.StatusInternalServerError
	}

	requestId := uint64(0)
	payload, err := readJSONRPCPayload(poktRequestBz.Body)
	if err == nil {
		requestId = payload.Id
	}

	errorReplyPayload := map[string]any{
		"jsonrpc": "2.0",
		"id":      requestId,
		"error": map[string]any{
			"code":    -32000,
			"message": errorMsg,
			"data":    nil,
		},
	}

	responseBodyBz, err := json.Marshal(errorReplyPayload)
	if err != nil {
		return defaultJSONRPCErrorReply, defaultJSONRPCErrorReplyBz
	}

	poktResponse := &httpcodec.HTTPResponse{
		StatusCode: int32(statusCode),
		Header:     map[string]string{contentTypeHeaderKey: "application/json"},
		Body:       responseBodyBz,
	}

	responseBz, err := proto.Marshal(poktResponse)
	if err != nil {
		return defaultJSONRPCErrorReply, defaultJSONRPCErrorReplyBz
	}

	return poktResponse, responseBz
}

func readJSONRPCPayload(requestBodyBz []byte) (*jsonRPCPayload, error) {
	var payload jsonRPCPayload
	if err := json.Unmarshal(requestBodyBz, &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}
