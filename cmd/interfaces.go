package cmd

import (
	"context"
	"io"
	"net/http"
)

// RESTClientInterface defines the common interface for REST clients
// This allows us to use both the real api.RESTClient and our MockRESTClient in tests
type RESTClientInterface interface {
	Get(path string, response interface{}) error
	Post(path string, body io.Reader, response interface{}) error
	Put(path string, body io.Reader, response interface{}) error
	Patch(path string, body io.Reader, response interface{}) error
	Delete(path string, response interface{}) error
	Do(method string, path string, body io.Reader, response interface{}) error
	DoWithContext(ctx context.Context, method string, path string, body io.Reader, response interface{}) error
	Request(method string, path string, body io.Reader) (*http.Response, error)
	RequestWithContext(ctx context.Context, method string, path string, body io.Reader) (*http.Response, error)
}