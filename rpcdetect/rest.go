package rpcdetect

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

func isREST(_ *http.Request) bool {
	return true
}

func formatRESTError(request *http.Request, err error, isInternal bool) *http.Response {
	statusCode := http.StatusBadRequest
	errorMsg := err.Error()

	if isInternal {
		statusCode = http.StatusInternalServerError
		errorMsg = "Internal error"
	}

	if request.Header.Get("Content-Type") == "application/json" {
		errorMsg = fmt.Sprintf(`{"error": "%s"}`, errorMsg)
	}

	headers := http.Header{}
	headers.Set("Content-Type", request.Header.Get("Content-Type"))

	bodyReader := io.NopCloser(bytes.NewReader([]byte(errorMsg)))

	return &http.Response{
		StatusCode: statusCode,
		Header:     headers,
		Body:       bodyReader,
	}
}
