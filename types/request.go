package types

import (
	"io"
	"net/http"
	"slices"

	"google.golang.org/protobuf/proto"
)

// SerializeHTTPRequest take an http.Request object and serializes it into a byte
// slice that can be embedded into another struct, such as RelayRequest.Payload.
func SerializeHTTPRequest(
	request *http.Request,
) (poktHTTPRequest *POKTHTTPRequest, poktHTTPRequestBz []byte, err error) {
	requestBodyBz, err := io.ReadAll(request.Body)
	request.Body.Close()
	if err != nil {
		return nil, nil, err
	}

	headers := map[string]*Header{}
	for key := range request.Header {
		// Sort the header values to ensure that the order of the values is
		// consistent and byte-for-byte equal when comparing the serialized
		// request.
		headerValues := request.Header.Values(key)
		slices.Sort(headerValues)
		headers[key] = &Header{
			Key:    key,
			Values: headerValues,
		}
	}

	poktHTTPRequest = &POKTHTTPRequest{
		Method: request.Method,
		Header: headers,
		Url:    request.URL.String(),
		BodyBz: requestBodyBz,
	}

	poktHTTPRequestBz, err = proto.Marshal(poktHTTPRequest)

	return poktHTTPRequest, poktHTTPRequestBz, err
}

// DeserializeHTTPRequest takes a byte slice and deserializes it into a
// POKTHTTPRequest object.
func DeserializeHTTPRequest(requestBz []byte) (request *POKTHTTPRequest, err error) {
	poktHTTPRequest := &POKTHTTPRequest{}

	if err := proto.Unmarshal(requestBz, poktHTTPRequest); err != nil {
		return nil, err
	}

	if poktHTTPRequest.Header == nil {
		poktHTTPRequest.Header = map[string]*Header{}
	}

	return poktHTTPRequest, err
}
