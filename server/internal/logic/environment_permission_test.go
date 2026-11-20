package logic

import (
	"context"
	"errors"
	"testing"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type matrixEnvironments struct {
	repository.EnvironmentRepository
}

func (matrixEnvironments) FindByID(context.Context, int64) (*model.Environment, error) {
	return &model.Environment{ProjectID: 7}, nil
}

type matrixMembers struct {
	repository.ProjectMemberRepository
	projectID int64
}

func (m matrixMembers) Get(_ context.Context, projectID, _ int64) (*model.ProjectMember, error) {
	if projectID != m.projectID {
		return nil, repository.Missing("member")
	}
	return &model.ProjectMember{Role: "OWNER"}, nil
}

type matrixGrants struct {
	items  []model.EnvironmentPermissionGrant
	locked bool
}

func (g *matrixGrants) LockScope(context.Context, int64) error { g.locked = true; return nil }
func (g *matrixGrants) List(context.Context, int64) ([]model.EnvironmentPermissionGrant, error) {
	return g.items, nil
}
func (g *matrixGrants) Replace(_ context.Context, envID int64, role string, points []string, actorID int64) error {
	g.items = nil
	for _, p := range points {
		g.items = append(g.items, model.EnvironmentPermissionGrant{EnvironmentID: envID, Role: role, Permission: p, CreatedBy: actorID})
	}
	return nil
}

type matrixTX struct{}

func (matrixTX) TX(ctx context.Context, f func(context.Context) error) error { return f(ctx) }
func TestMatrixProjectBoundaryAndReplacement(t *testing.T) {
	grants := &matrixGrants{}
	logic := NewEnvironmentPermissionLogic(grants, matrixEnvironments{}, matrixMembers{projectID: 8}, matrixTX{})
	_, err := logic.Update(context.Background(), permission.Actor{UserID: 1}, 2, nil)
	if !errors.Is(err, permission.ErrForbidden) {
		t.Fatalf("cross-project access: %v", err)
	}
	logic.members = matrixMembers{projectID: 7}
	for _, points := range [][]string{{"READ", "WRITE"}, {}, {"WRITE"}} {
		result, err := logic.Update(context.Background(), permission.Actor{UserID: 1}, 2, []*adminv1.EnvironmentPermissionRole{{Role: "MEMBER", Permissions: points}})
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 4 || len(result[2].Permissions) != len(points) || len(result[0].Permissions) != 4 || !grants.locked {
			t.Fatalf("unexpected matrix: %v", result)
		}
	}
}
