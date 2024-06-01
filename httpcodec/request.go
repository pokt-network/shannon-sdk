package httpcodec

import (
	"io"
	"net/http"
	"strings"

	"google.golang.org/protobuf/proto"
)

// SerializeHTTPRequest take an http.Request object and serializes it into a byte
// slice that can be embedded into another struct, such as RelayRequest.Payload.
func SerializeHTTPRequest(request *http.Request) (body []byte, err error) {
	requestBodyBz, err := io.ReadAll(request.Body)
	request.Body.Close()
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	for key := range request.Header {
		headers[key] = strings.Join(request.Header.Values(key), ",")
	}

	httpRequest := &HTTPRequest{
		Method: request.Method,
		Header: headers,
		Url:    request.URL.String(),
		Body:   requestBodyBz,
	}

	return proto.Marshal(httpRequest)
}

// DeserializeHTTPRequest takes a byte slice and deserializes it into a
// SerializableHTTPRequest object.
func DeserializeHTTPRequest(requestBz []byte) (request *HTTPRequest, err error) {
	httpRequest := &HTTPRequest{}

	if err := proto.Unmarshal(requestBz, httpRequest); err != nil {
		return nil, err
	}

	return httpRequest, nil
}
