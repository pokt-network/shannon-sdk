package httpcodec

import (
	"io"
	"net/http"
	"strings"

	"google.golang.org/protobuf/proto"
)

// SerializeHTTPResponse take an http.Response object and serializes it into a byte
// slice that can be embedded into another struct, such as RelayResponse.Payload.
func SerializeHTTPResponse(response *http.Response) (body []byte, err error) {
	responseBodyBz, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	for key := range response.Header {
		headers[key] = strings.Join(response.Header.Values(key), ",")
	}

	httpResponse := &HTTPResponse{
		StatusCode: int32(response.StatusCode),
		Header:     headers,
		Body:       responseBodyBz,
	}

	return proto.Marshal(httpResponse)
}

// DeserializeHTTPResponse takes a byte slice and deserializes it into a
// SerializableHTTPResponse object.
func DeserializeHTTPResponse(responseBz []byte) (response *HTTPResponse, err error) {
	httpResponse := &HTTPResponse{}

	if err := proto.Unmarshal(responseBz, httpResponse); err != nil {
		return nil, err
	}

	return httpResponse, nil
}
