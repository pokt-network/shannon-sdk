package rpcdetect

import (
	"fmt"
	"net/http"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/pokt-network/shannon-sdk/types"
)

var (
	// defaultRESTErrorReply is the default REST error reply to be sent if the
	// formatRESTError function fails to format the appropriate one.
	defaultRESTErrorReply   *types.POKTHTTPResponse
	defaultRESTErrorReplyBz []byte
)

// init initializes the default REST error reply. This function is called before
// the main function and panics if it fails to marshal the default REST error reply,
// making the program exit early.
func init() {
	// Initialize the default REST error reply
	header := &types.Header{
		Key:    contentTypeHeaderKey,
		Values: []string{"text/plain"},
	}
	headers := map[string]*types.Header{contentTypeHeaderKey: header}

	defaultRESTErrorReply = &types.POKTHTTPResponse{
		StatusCode: http.StatusInternalServerError,
		Header:     headers,
		BodyBz:     []byte(`Internal error`),
	}

	var err error
	defaultRESTErrorReplyBz, err = proto.Marshal(defaultRESTErrorReply)
	if err != nil {
		panic(err)
	}
}

// isREST checks if the given POKTHTTPRequest is a REST request.
func isREST(poktRequest *types.POKTHTTPRequest) bool {
	return poktRequest.Url != ""
}

// formatRESTError formats the given error into a POKTHTTPResponse and its
// corresponding byte representation.
func formatRESTError(
	err error,
	poktRequest *types.POKTHTTPRequest,
	isInternal bool,
) (*types.POKTHTTPResponse, []byte) {
	errorMsg := err.Error()
	statusCode := http.StatusBadRequest
	if isInternal {
		errorMsg = "Internal error"
		statusCode = http.StatusInternalServerError
	}

	contentTypeHeaderValues := []string{"text/plain"}
	if contentTypeHeader, ok := poktRequest.Header[contentTypeHeaderKey]; ok {
		contentTypeHeaderValues = contentTypeHeader.Values
	}

	responseBodyBz := []byte(errorMsg)
	if slices.Contains(contentTypeHeaderValues, "application/json") {
		responseBodyBz = []byte(fmt.Sprintf(`{"error": "%s"}`, errorMsg))
	}

	header := &types.Header{
		Key:    contentTypeHeaderKey,
		Values: contentTypeHeaderValues,
	}
	headers := map[string]*types.Header{contentTypeHeaderKey: header}

	poktResponse := &types.POKTHTTPResponse{
		StatusCode: uint32(statusCode),
		Header:     headers,
		BodyBz:     responseBodyBz,
	}

	poktResponseBz, err := proto.Marshal(poktResponse)
	if err != nil {
		return defaultRESTErrorReply, defaultRESTErrorReplyBz
	}

	return poktResponse, poktResponseBz
}
