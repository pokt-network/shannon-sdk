package httpcodec

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
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
	for key, values := range request.Header {
		headers[key] = strings.Join(values, ",")
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
func DeserializeHTTPRequest(requestBz []byte) (request *http.Request, err error) {
	httpRequest := &HTTPRequest{}

	if err := proto.Unmarshal(requestBz, httpRequest); err != nil {
		return nil, err
	}

	headers := make(http.Header)
	for key, valuesStr := range httpRequest.Header {
		values := strings.Split(valuesStr, ",")
		for _, value := range values {
			headers.Add(key, value)
		}
	}

	requestUrl, err := url.Parse(httpRequest.Url)
	if err != nil {
		return nil, err
	}

	request = &http.Request{
		Method: httpRequest.Method,
		Header: headers,
		URL:    requestUrl,
		Body:   io.NopCloser(bytes.NewReader(httpRequest.Body)),
	}

	return request, nil
}
