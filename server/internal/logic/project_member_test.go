package logic

import (
	"context"
	"errors"
	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/repository"
	"net/http"
	"testing"

	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	jwtlib "github.com/golang-jwt/jwt/v5"

	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	adminmiddleware "github.com/yvvlee/kirby/server/internal/middleware"
	"github.com/yvvlee/kirby/server/internal/model"
)

type actorVerifier struct{}

func (actorVerifier) Verify(string) (*authjwt.Claims, error) {
	return &authjwt.Claims{SessionID: "session", RegisteredClaims: jwtlib.RegisteredClaims{Subject: "7"}}, nil
}

type actorUsers struct{}

func (actorUsers) GetByID(context.Context, int64) (*model.User, error) {
	return &model.User{Meta: model.Meta{ID: 7}, Enabled: true}, nil
}

type actorTransport struct{ header actorHeader }

func (actorTransport) Kind() transport.Kind              { return transport.KindHTTP }
func (actorTransport) Endpoint() string                  { return "http://localhost" }
func (actorTransport) Operation() string                 { return "test" }
func (t actorTransport) RequestHeader() transport.Header { return t.header }
func (actorTransport) ReplyHeader() transport.Header     { return actorHeader{} }

type actorHeader http.Header

func (h actorHeader) Get(key string) string      { return http.Header(h).Get(key) }
func (h actorHeader) Set(key, value string)      { http.Header(h).Set(key, value) }
func (h actorHeader) Add(key, value string)      { http.Header(h).Add(key, value) }
func (h actorHeader) Values(key string) []string { return http.Header(h).Values(key) }
func (h actorHeader) Keys() []string {
	result := make([]string, 0, len(h))
	for key := range h {
		result = append(result, key)
	}
	return result
}

func membershipContext(t *testing.T) context.Context {
	header := actorHeader{}
	header.Set("Authorization", "Bearer token")
	ctx := transport.NewServerContext(context.Background(), actorTransport{header: header})
	var result context.Context
	_, err := adminmiddleware.AdminAuth(actorVerifier{}, actorUsers{})(kratosmiddleware.Handler(func(ctx context.Context, _ any) (any, error) { result = ctx; return nil, nil }))(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

type memberRejectRepo struct {
	repository.ProjectMemberRepository
	actor    string
	target   string
	getErr   error
	countErr error
}

func (r memberRejectRepo) Get(_ context.Context, projectID, userID int64) (*model.ProjectMember, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if projectID != 1 {
		return nil, repository.Missing("member")
	}
	role := r.target
	if userID == 7 {
		role = r.actor
	}
	return &model.ProjectMember{Role: role}, nil
}
func (r memberRejectRepo) CountOwners(context.Context, int64) (int64, error) { return 1, r.countErr }
func TestMemberMutationsRejectUnauthorizedTargets(t *testing.T) {
	ctx := membershipContext(t)
	cases := []struct {
		name    string
		actor   string
		target  string
		project int64
	}{
		{"member", "MEMBER", "GUEST", 1},
		{"guest", "GUEST", "GUEST", 1},
		{"maintainer-owner", "MAINTAINER", "OWNER", 1},
		{"last-owner", "OWNER", "OWNER", 1},
		{"other-project", "OWNER", "GUEST", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := &ProjectMemberLogic{members: memberRejectRepo{actor: tc.actor, target: tc.target}}
			_, err := l.mutateUpdateProjectMemberRole(ctx, &adminv1.UpdateProjectMemberRoleRequest{ProjectId: tc.project, UserId: 8, Role: "GUEST"})
			if err == nil {
				t.Fatal("role mutation allowed")
			}
			_, err = l.mutateRemoveProjectMember(ctx, &adminv1.ProjectMemberTargetRequest{ProjectId: tc.project, UserId: 8})
			if err == nil {
				t.Fatal("removal allowed")
			}
		})
	}
}
func TestMemberMutationPreservesDatabaseError(t *testing.T) {
	expected := errors.New("database unavailable")
	l := &ProjectMemberLogic{members: memberRejectRepo{getErr: expected}}
	_, err := l.mutateAddProjectMember(membershipContext(t), &adminv1.AddProjectMemberRequest{ProjectId: 1, UserId: 8, Role: "GUEST"})
	if !errors.Is(err, expected) {
		t.Fatalf("error lost: %v", err)
	}
}
