package types

import (
	"fmt"
	"net/http"
	"slices"

	"google.golang.org/protobuf/proto"
)

var (
	// defaultRESTErrorReply is the default REST error reply to be sent if the
	// formatRESTError function fails to format the appropriate one.
	defaultRESTErrorReply   *POKTHTTPResponse
	defaultRESTErrorReplyBz []byte
)

// init initializes the package level variables such as the REST error reply.
func init() {
	// Initialize the default REST error reply and panic if it fails. This is done
	// to make the program exit early if the default REST error reply fails to be
	// marshaled.
	initDefaultRESTErrorReply()
}

// isREST checks if the given POKTHTTPRequest is a REST request.
func (poktRequest *POKTHTTPRequest) isREST() bool {
	// Since a REST request could have an empty body, can be of any method, and
	// can have any content type, we can't use those to determine if a request is
	// a REST request.
	// One general way to determine if a request is a REST request is to check if
	// it has a non-empty URL. This is because REST requests have typically at least
	// one URL path segment, which could be the resource path, api version, etc.
	return poktRequest.Url != ""
}

// formatRESTError formats the given error into a POKTHTTPResponse and its
// corresponding byte representation.
func (poktRequest *POKTHTTPRequest) formatRESTError(
	err error,
	isInternal bool,
) (*POKTHTTPResponse, []byte) {
	errorMsg := err.Error()
	statusCode := http.StatusBadRequest
	if isInternal {
		errorMsg = defaultErrorMessage
		statusCode = http.StatusInternalServerError
	}

	contentTypeHeaderValues := []string{contentTypeHeaderValueText}
	if contentTypeHeader, ok := poktRequest.Header[contentTypeHeaderKey]; ok {
		contentTypeHeaderValues = contentTypeHeader.Values
	}

	responseBodyBz := []byte(errorMsg)
	if slices.Contains(contentTypeHeaderValues, contentTypeHeaderValueJSON) {
		responseBodyBz = []byte(fmt.Sprintf(`{"error": "%s"}`, errorMsg))
	}

	header := &Header{
		Key:    contentTypeHeaderKey,
		Values: contentTypeHeaderValues,
	}
	headers := map[string]*Header{contentTypeHeaderKey: header}

	poktResponse := &POKTHTTPResponse{
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

// initDefaultRESTErrorReply initializes the default REST error reply.
// This function is called before the main function and panics if it fails to
// marshal the default REST error reply, making the program exit early.
func initDefaultRESTErrorReply() {
	// Initialize the default REST error reply
	header := &Header{
		Key:    contentTypeHeaderKey,
		Values: []string{contentTypeHeaderValueText},
	}
	headers := map[string]*Header{contentTypeHeaderKey: header}

	defaultRESTErrorReply = &POKTHTTPResponse{
		StatusCode: http.StatusInternalServerError,
		Header:     headers,
		BodyBz:     []byte(defaultErrorMessage),
	}

	var err error
	defaultRESTErrorReplyBz, err = proto.Marshal(defaultRESTErrorReply)
	if err != nil {
		panic(err)
	}
}
