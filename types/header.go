package types

import "net/http"

// TODO_REFACTOR: Move these helper functions to a more appropriate package.

// CopyToHTTPHeader copies the POKTHTTPRequest header map to the httpHeader map.
func (req *POKTHTTPRequest) CopyToHTTPHeader(httpHeader http.Header) {
	for key, header := range req.Header {
		for _, value := range header.Values {
			httpHeader.Add(key, value)
		}
	}
}

// CopyToHTTPHeader copies the POKTHTTPResponse header map to the httpHeader map.
func (req *POKTHTTPResponse) CopyToHTTPHeader(httpHeader http.Header) {
	for key, header := range req.Header {
		for _, value := range header.Values {
			httpHeader.Add(key, value)
		}
	}
}
