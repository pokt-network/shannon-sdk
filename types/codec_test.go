package types_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/pokt-network/shannon-sdk/types"
)

var (
	contentTypeHeaderKey       = "Content-Type"
	contentTypeHeaderValueJSON = "application/json"
	arbitraryHeaderKey         = "Arbitrary-Key"
	arbitraryHeaderFirstValue  = "arbitrary-first-value"
	arbitraryHeaderSecondValue = "arbitrary-second-value"
	contentBz                  = []byte(`{"key":"value"}`)
	contentUrl                 = "http://localhost:8080"
	requestMethod              = "POST"
)

func TestCodec_SerializeRequest_Success(t *testing.T) {
	requestUrl, err := url.Parse(contentUrl)
	require.NoError(t, err)

	req := &http.Request{
		Method: requestMethod,
		Header: map[string][]string{
			contentTypeHeaderKey: {contentTypeHeaderValueJSON},
			arbitraryHeaderKey:   {arbitraryHeaderFirstValue, arbitraryHeaderSecondValue},
		},
		URL:  requestUrl,
		Body: io.NopCloser(bytes.NewReader(contentBz)),
	}

	poktReq, poktReqBz, err := types.SerializeHTTPRequest(req)
	require.NoError(t, err)

	opts := proto.MarshalOptions{Deterministic: true}
	marshalledPOKTReqBz, err := opts.Marshal(poktReq)
	require.NoError(t, err)

	for key := range req.Header {
		for i, value := range req.Header.Values(key) {
			require.Equal(t, value, poktReq.Header[key].Values[i])
		}
	}
	require.Equal(t, req.Method, poktReq.Method)
	require.Equal(t, req.URL.String(), poktReq.Url)
	require.Equal(t, poktReq.BodyBz, contentBz)
	require.Equal(t, poktReqBz, marshalledPOKTReqBz)
}

func TestCodec_SerializeRequest_Error(t *testing.T) {
	requestUrl, err := url.Parse(contentUrl)
	require.NoError(t, err)

	req := &http.Request{
		Method: requestMethod,
		Header: map[string][]string{
			contentTypeHeaderKey: {contentTypeHeaderValueJSON},
		},
		URL:  requestUrl,
		Body: io.NopCloser(&errorReader{}),
	}

	_, _, err = types.SerializeHTTPRequest(req)
	require.Error(t, err)
}

func TestCodec_DeserializeRequest_Success(t *testing.T) {
	req := &http.Request{
		Method: requestMethod,
		Header: map[string][]string{
			contentTypeHeaderKey: {contentTypeHeaderValueJSON},
			arbitraryHeaderKey:   {arbitraryHeaderFirstValue, arbitraryHeaderSecondValue},
		},
		URL:  &url.URL{Path: contentUrl},
		Body: io.NopCloser(bytes.NewReader(contentBz)),
	}

	poktReq, poktReqBz, err := types.SerializeHTTPRequest(req)
	require.NoError(t, err)

	deserializedPOKTReq, err := types.DeserializeHTTPRequest(poktReqBz)
	require.NoError(t, err)

	require.Equal(t, poktReq.BodyBz, deserializedPOKTReq.BodyBz)
	require.Equal(t, poktReq.Url, deserializedPOKTReq.Url)
	require.Equal(t, poktReq.Method, deserializedPOKTReq.Method)
	for key, header := range poktReq.Header {
		for i, value := range header.Values {
			require.Equal(t, value, deserializedPOKTReq.Header[key].Values[i])
		}
	}
}

func TestCodec_DeserializeRequest_Error(t *testing.T) {
	_, err := types.DeserializeHTTPRequest([]byte("invalid"))
	require.Error(t, err)
}

func TestCodec_SerializeResponse_Success(t *testing.T) {
	res := &http.Response{
		StatusCode: http.StatusOK,
		Header: map[string][]string{
			contentTypeHeaderKey: {contentTypeHeaderValueJSON},
			arbitraryHeaderKey:   {arbitraryHeaderFirstValue, arbitraryHeaderSecondValue},
		},
		Body: io.NopCloser(bytes.NewReader(contentBz)),
	}

	poktRes, poktResBz, err := types.SerializeHTTPResponse(res)
	require.NoError(t, err)

	opts := proto.MarshalOptions{Deterministic: true}
	marshalledPOKTResBz, err := opts.Marshal(poktRes)
	require.NoError(t, err)

	for key := range res.Header {
		for i, value := range res.Header.Values(key) {
			require.Equal(t, value, poktRes.Header[key].Values[i])
		}
	}
	require.Equal(t, res.StatusCode, int(poktRes.StatusCode))
	require.Equal(t, poktRes.BodyBz, contentBz)
	require.Equal(t, poktResBz, marshalledPOKTResBz)
}

func TestCodec_SerializeResponse_Error(t *testing.T) {
	res := &http.Response{
		StatusCode: http.StatusOK,
		Header: map[string][]string{
			contentTypeHeaderKey: {contentTypeHeaderValueJSON},
		},
		Body: io.NopCloser(&errorReader{}),
	}

	_, _, err := types.SerializeHTTPResponse(res)
	require.Error(t, err)
}

func TestCodec_DeserializeResponse_Success(t *testing.T) {
	res := &http.Response{
		StatusCode: http.StatusOK,
		Header: map[string][]string{
			contentTypeHeaderKey: {contentTypeHeaderValueJSON},
			arbitraryHeaderKey:   {arbitraryHeaderFirstValue, arbitraryHeaderSecondValue},
		},
		Body: io.NopCloser(bytes.NewReader(contentBz)),
	}

	poktRes, poktResBz, err := types.SerializeHTTPResponse(res)
	require.NoError(t, err)

	deserializedPOKTRes, err := types.DeserializeHTTPResponse(poktResBz)
	require.NoError(t, err)

	require.Equal(t, poktRes.BodyBz, deserializedPOKTRes.BodyBz)
	require.Equal(t, poktRes.StatusCode, uint32(deserializedPOKTRes.StatusCode))
	for key, header := range poktRes.Header {
		for i, value := range header.Values {
			require.Equal(t, value, deserializedPOKTRes.Header[key].Values[i])
		}
	}
}

func TestCodec_DeserializeResponse_Error(t *testing.T) {
	_, err := types.DeserializeHTTPResponse([]byte("invalid"))
	require.Error(t, err)
}

// errorReader is an io.Reader that always returns an error.
type errorReader struct{}

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("test error")
}
