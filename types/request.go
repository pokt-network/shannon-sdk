package types

import (
	"io"
	"net/http"

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
		headerValues := request.Header.Values(key)
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

	// Use deterministic marshalling to ensure that the serialized request is
	// byte-for-byte equal when comparing the serialized request.
	opts := proto.MarshalOptions{Deterministic: true}

	poktHTTPRequestBz, err = opts.Marshal(poktHTTPRequest)

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
