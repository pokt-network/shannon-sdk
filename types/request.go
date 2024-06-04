package types

import (
	"bytes"
	"io"
	"net/http"
	"net/url"

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

	headers := map[string]*Header{}
	for key := range request.Header {
		headers[key] = &Header{
			Key:    key,
			Values: request.Header.Values(key),
		}
	}

	httpRequest := &POKTHTTPRequest{
		Method: request.Method,
		Header: headers,
		Url:    request.URL.String(),
		BodyBz: requestBodyBz,
	}

	return proto.Marshal(httpRequest)
}

// DeserializeHTTPRequest takes a byte slice and deserializes it into a
// SerializableHTTPRequest object.
func DeserializeHTTPRequest(requestBz []byte) (request *http.Request, err error) {
	poktHTTPRequest := &POKTHTTPRequest{}

	if err := proto.Unmarshal(requestBz, poktHTTPRequest); err != nil {
		return nil, err
	}

	headers := make(http.Header)
	for key, header := range poktHTTPRequest.Header {
		for _, value := range header.Values {
			headers.Add(key, value)
		}
	}

	requestUrl, err := url.Parse(poktHTTPRequest.Url)
	if err != nil {
		return nil, err
	}

	request = &http.Request{
		Method: poktHTTPRequest.Method,
		Header: headers,
		URL:    requestUrl,
		Body:   io.NopCloser(bytes.NewReader(poktHTTPRequest.BodyBz)),
	}

	return request, nil
}
