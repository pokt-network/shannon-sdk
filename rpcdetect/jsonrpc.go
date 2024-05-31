package rpcdetect

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

var internalErrorPayload = []byte(`{"jsonrpc":"2.0","id":0,"error":{"code":-32000,"message":"Internal error","data":null}}`)

type jsonRPCPayload struct {
	Id      uint64 `json:"id"`
	JsonRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
}

func isJSONRPC(request *http.Request) bool {
	header := request.Header
	if header.Get("Content-Type") != "application/json" {
		return false
	}

	payload, err := readJSONRPCPayload(request)
	if err != nil {
		return false
	}

	if payload.Id == 0 || len(payload.JsonRPC) == 0 || len(payload.Method) == 0 {
		return false
	}

	return true
}

func formatJSONRPCError(request *http.Request, err error, isInternal bool) *http.Response {
	errStr := err.Error()
	if isInternal {
		errStr = "Internal error"
	}

	requestId := uint64(0)
	payload, err := readJSONRPCPayload(request)
	if err == nil {
		requestId = payload.Id
	}

	errorReplayPayload := map[string]any{
		"jsonrpc": "2.0",
		"id":      requestId,
		"error": map[string]any{
			"code":    -32000,
			"message": errStr,
			"data":    nil,
		},
	}

	responseBody, err := json.Marshal(errorReplayPayload)
	if err != nil {
		responseBody = internalErrorPayload
	}

	bodyReader := io.NopCloser(bytes.NewReader(responseBody))

	return &http.Response{Body: bodyReader}
}

func readJSONRPCPayload(request *http.Request) (*jsonRPCPayload, error) {
	payloadBz, err := io.ReadAll(request.Body)
	request.Body.Close()
	if err != nil {
		return nil, err
	}
	request.Body = io.NopCloser(bytes.NewReader(payloadBz))

	var payload jsonRPCPayload
	if err = json.Unmarshal(payloadBz, &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}
