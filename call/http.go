package call

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type HTTPResponse struct {
	StatusCode int
	Body       interface{}
	RawBody    []byte
	Error      error
}

func MakeHTTPRequest(
	url string,
	method string,
	headers map[string]string,
	body interface{},
) (*HTTPResponse, error) {
	var requestBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize request body: %w", err)
		}
		requestBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	if body != nil && (method == http.MethodPost || method == http.MethodPut) {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var responseBody interface{}
	if err := json.Unmarshal(rawBody, &responseBody); err != nil {
		responseBody = string(rawBody)
	}

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Body:       responseBody,
		RawBody:    rawBody,
		Error:      nil,
	}, nil
}
