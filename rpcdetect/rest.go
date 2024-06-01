package rpcdetect

import (
	"fmt"
	"net/http"

	"google.golang.org/protobuf/proto"

	"github.com/pokt-network/shannon-sdk/httpcodec"
)

var (
	defaultRESTErrorReply   *httpcodec.HTTPResponse
	defaultRESTErrorReplyBz []byte
)

func init() {
	defaultRESTErrorReply = &httpcodec.HTTPResponse{
		StatusCode: http.StatusInternalServerError,
		Header:     map[string]string{contentTypeHeaderKey: "text/plain"},
		Body:       []byte(`Internal error`),
	}

	var err error
	defaultRESTErrorReplyBz, err = proto.Marshal(defaultRESTErrorReply)
	if err != nil {
		panic(err)
	}
}

func isREST(_ *httpcodec.HTTPRequest) bool {
	return true
}

func formatRESTError(
	err error,
	poktRequest *httpcodec.HTTPRequest,
	isInternal bool,
) (*httpcodec.HTTPResponse, []byte) {
	errorMsg := err.Error()
	statusCode := http.StatusBadRequest
	if isInternal {
		errorMsg = "Internal error"
		statusCode = http.StatusInternalServerError
	}

	contentTypeHeaderValue := poktRequest.Header[contentTypeHeaderKey]
	responseBodyBz := []byte(errorMsg)
	if contentTypeHeaderValue == "application/json" {
		responseBodyBz = []byte(fmt.Sprintf(`{"error": "%s"}`, errorMsg))
	}

	poktResponse := &httpcodec.HTTPResponse{
		StatusCode: int32(statusCode),
		Header:     map[string]string{contentTypeHeaderKey: contentTypeHeaderValue},
		Body:       responseBodyBz,
	}

	poktResponseBz, err := proto.Marshal(poktResponse)
	if err != nil {
		return defaultRESTErrorReply, defaultRESTErrorReplyBz
	}

	return poktResponse, poktResponseBz
}
