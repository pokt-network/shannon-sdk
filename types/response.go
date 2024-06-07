package types

import (
	"io"
	"net/http"

	"google.golang.org/protobuf/proto"
)

// SerializeHTTPResponse take an http.Response object and serializes it into a byte
// slice that can be embedded into another struct, such as RelayResponse.Payload.
func SerializeHTTPResponse(
	response *http.Response,
) (poktHTTPResponse *POKTHTTPResponse, poktHTTPResponseBz []byte, err error) {
	responseBodyBz, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, err
	}
	response.Body.Close()

	headers := map[string]*Header{}
	// http.Header is a map of header keys to a list of values. We need to get
	// the http.Header.Values(key) to get all the values of a key.
	// We have to avoid using http.Header.Get(key) because it only returns the
	// first value of the key.
	for key := range response.Header {
		headers[key] = &Header{
			Key:    key,
			Values: response.Header.Values(key),
		}
	}

	poktHTTPResponse = &POKTHTTPResponse{
		StatusCode: uint32(response.StatusCode),
		Header:     headers,
		BodyBz:     responseBodyBz,
	}

	poktHTTPResponseBz, err = proto.Marshal(poktHTTPResponse)

	return poktHTTPResponse, poktHTTPResponseBz, err
}

// DeserializeHTTPResponse takes a byte slice and deserializes it into a
// SerializableHTTPResponse object.
func DeserializeHTTPResponse(responseBz []byte) (response *POKTHTTPResponse, err error) {
	poktHTTPResponse := &POKTHTTPResponse{}

	if err := proto.Unmarshal(responseBz, poktHTTPResponse); err != nil {
		return nil, err
	}

	if poktHTTPResponse.Header == nil {
		poktHTTPResponse.Header = map[string]*Header{}
	}

	return poktHTTPResponse, err
}
