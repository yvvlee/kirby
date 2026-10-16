package service

import (
	"context"
	"testing"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
)

func TestCreateUserRequiresAuthentication(t *testing.T) {
	service := &UserService{}
	_, err := service.CreateUser(context.Background(), &adminv1.CreateUserRequest{
		Username: "bob",
		Password: "password123",
	})
	if err == nil {
		t.Fatal("expected error without actor context")
	}
}
