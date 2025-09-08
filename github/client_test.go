package github

import (
	"testing"
)

func TestInitClient(t *testing.T) {
	token := "test-token"
	client := InitClient(token)

	if client == nil {
		t.Error("InitClient() returned nil client")
	}

	// Verify that the client is properly configured
	// We can't easily test the token directly, but we can check that the client was created
	if client.Organizations == nil {
		t.Error("Client organizations service is nil")
	}

	if client.Users == nil {
		t.Error("Client users service is nil")
	}

	if client.Teams == nil {
		t.Error("Client teams service is nil")
	}

	if client.Repositories == nil {
		t.Error("Client repositories service is nil")
	}
}

func TestInitClientWithEmptyToken(t *testing.T) {
	// Even with empty token, client should be created (token validation happens at API call time)
	token := ""
	client := InitClient(token)

	if client == nil {
		t.Error("InitClient() returned nil client even with empty token")
	}
}
