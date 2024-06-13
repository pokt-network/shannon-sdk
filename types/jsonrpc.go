package types

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"google.golang.org/protobuf/proto"
)

const (
	// defaultJSONRPCErrorCode is the default JSON-RPC error code to be used when
	// generating a JSON-RPC error reply.
	// JSON-RPC specification uses -32000 to -32099 as implementation-defined server-errors.
	// See: https://www.jsonrpc.org/specification#error_object
	defaultJSONRPCErrorCode = -32000
)

var (
	// defaultJSONRPCErrorReply is the default JSON-RPC error reply to be sent if the
	// formatJSONRPCError function fails to format the appropriate one.
	defaultJSONRPCErrorReply   *POKTHTTPResponse
	defaultJSONRPCErrorReplyBz []byte
)

// jsonRPCPayloadMeta represents the JSON-RPC payload fields that are relevant for
// detecting JSON-RPC requests.
type jsonRPCPayloadMeta struct {
	Id      uint64 `json:"id"`
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
}

// init initializes the package level variables such as the JSON-RPC error reply.
func init() {
	// Initialize the default JSON-RPC error reply and panic if it fails. This is done
	// to make the program exit early if the default JSON-RPC error reply fails to be
	// marshaled.
	initDefaultJSONRPCErrorReply()
}

// isJSONRPC checks if the given POKTHTTPRequest is a JSON-RPC request.
func (poktRequest *POKTHTTPRequest) isJSONRPC() bool {
	contentType, ok := poktRequest.Header[contentTypeHeaderKey]
	if !ok {
		return false
	}

	if !slices.Contains(contentType.Values, contentTypeHeaderValueJSON) {
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
func (poktRequestBz *POKTHTTPRequest) formatJSONRPCError(
	err error,
	isInternal bool,
) (*POKTHTTPResponse, []byte) {
	errorMsg := err.Error()
	// If the error is internal, we don't to expose the error message to the client
	// but instead return a generic error message.
	if isInternal {
		errorMsg = defaultErrorMessage
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
			"code":    defaultJSONRPCErrorCode,
			"message": errorMsg,
			"data":    nil,
		},
	}

	responseBodyBz, err := json.Marshal(errorReplyPayload)
	if err != nil {
		// If we fail to marshal the error reply payload, we return the default JSON-RPC
		// error reply.
		return defaultJSONRPCErrorReply, defaultJSONRPCErrorReplyBz
	}

	header := &Header{
		Key:    contentTypeHeaderKey,
		Values: []string{contentTypeHeaderValueJSON},
	}
	// Headers are stored as a map of header key to Header structs that consist of the
	// key and the values as a slice of strings.
	headers := map[string]*Header{contentTypeHeaderKey: header}
	poktResponse := &POKTHTTPResponse{
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
func readJSONRPCPayload(requestBodyBz []byte) (*jsonRPCPayloadMeta, error) {
	var payload jsonRPCPayloadMeta
	if err := json.Unmarshal(requestBodyBz, &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}

// initDefaultJSONRPCErrorReply initializes the default JSON-RPC error reply.
// This function is called before the main function and panics if it fails to
// marshal the default JSON-RPC error reply, making the program exit early.
func initDefaultJSONRPCErrorReply() {
	// Initialize the default JSON-RPC error reply
	header := &Header{
		Key:    contentTypeHeaderKey,
		Values: []string{contentTypeHeaderValueJSON},
	}
	headers := map[string]*Header{contentTypeHeaderKey: header}

	defaultJSONRPCErrorReplyBody := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":0,"error":{"code":%d,"message":"%s","data":null}}`,
		defaultJSONRPCErrorCode,
		defaultErrorMessage,
	)

	defaultJSONRPCErrorReply = &POKTHTTPResponse{
		StatusCode: http.StatusInternalServerError,
		Header:     headers,
		BodyBz:     []byte(defaultJSONRPCErrorReplyBody),
	}

	var err error
	defaultJSONRPCErrorReplyBz, err = proto.Marshal(defaultJSONRPCErrorReply)
	if err != nil {
		panic(err)
	}
}
