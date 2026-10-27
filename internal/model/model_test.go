package model_test

import "kbase/internal/model"

// The external tests' own copies of the two fixtures the in-package tests
// keep in openai_client_test.go: an unexported name does not cross the package
// boundary between package model and package model_test, and these are all the
// two share.

const testAPIKey = "sk-TEST-MUST-NOT-LEAK-1234567890abcdef"

func newTestEndpoint(baseURL string) model.Endpoint {
	return model.Endpoint{
		Name:    "test",
		BaseURL: baseURL,
		APIKey:  testAPIKey,
	}
}
