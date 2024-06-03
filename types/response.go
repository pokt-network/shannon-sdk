package types

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"google.golang.org/protobuf/proto"
)

// SerializeHTTPResponse take an http.Response object and serializes it into a byte
// slice that can be embedded into another struct, such as RelayResponse.Payload.
func SerializeHTTPResponse(response *http.Response) (body []byte, err error) {
	responseBodyBz, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	response.Body.Close()

	header := make(map[string]string)
	// http.Header is a map of header keys to a list of values. We need to get
	// the http.Header.Values(key) to get all the values of a key, and then use
	// strings.Join prior assigning it to the map of header keys to a single
	// string.
	// We have to avoid using http.Header.Get(key) because it only returns the
	// first value of the key.
	// We do not use map<string, repeated string> because it is not supported by proto3
	// and requires an additional structure to represent the repeated string.
	for key := range response.Header {
		header[key] = strings.Join(response.Header.Values(key), ",")
	}

	poktHTTPResponse := &POKTHTTPResponse{
		StatusCode: uint32(response.StatusCode),
		Header:     header,
		BodyBz:     responseBodyBz,
	}

	return proto.Marshal(poktHTTPResponse)
}

// DeserializeHTTPResponse takes a byte slice and deserializes it into a
// SerializableHTTPResponse object.
func DeserializeHTTPResponse(responseBz []byte) (response *http.Response, err error) {
	poktHTTPResponse := &POKTHTTPResponse{}

	if err := proto.Unmarshal(responseBz, poktHTTPResponse); err != nil {
		return nil, err
	}

	header := make(http.Header)
	for key, valuesStr := range poktHTTPResponse.Header {
		// Split the values string by comma to get the list of values for the key is
		// holding, and then add each value to the http.Header.
		// Assigning the joined string to the http.Header will make it behave as a
		// single value, which is not the expected behavior.
		values := strings.Split(valuesStr, ",")
		for _, value := range values {
			header.Add(key, value)
		}
	}

	response = &http.Response{
		StatusCode: int(poktHTTPResponse.StatusCode),
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(poktHTTPResponse.BodyBz)),
	}

	return response, nil
}
