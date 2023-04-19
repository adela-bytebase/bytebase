package v1

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/store"
	v1pb "github.com/bytebase/bytebase/proto/generated-go/v1"
)

// RoleService implements the role service.
type RoleService struct {
	v1pb.UnimplementedRoleServiceServer
	store *store.Store
}

// NewRoleService returns a new instance of the role service.
func NewRoleService(store *store.Store) *RoleService {
	return &RoleService{store: store}
}

// ListRoles lists roles.
func (s *RoleService) ListRoles(ctx context.Context, _ *v1pb.ListRolesRequest) (*v1pb.ListRolesResponse, error) {
	roleMessages, err := s.store.ListRoles(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list roles: %v", err)
	}

	return &v1pb.ListRolesResponse{
		Roles: convertToRoles(roleMessages),
	}, nil
}

// CreateRole creates a new role.
func (s *RoleService) CreateRole(ctx context.Context, request *v1pb.CreateRoleRequest) (*v1pb.Role, error) {
	principalID := ctx.Value(common.PrincipalIDContextKey).(int)
	create := &store.RoleMessage{
		ResourceID:  request.RoleId,
		Description: request.Role.Description,
	}
	roleMessage, err := s.store.CreateRole(ctx, create, principalID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create role: %v", err)
	}
	return convertToRole(roleMessage), nil
}

// UpdateRole updates an existing role.
func (s *RoleService) UpdateRole(ctx context.Context, request *v1pb.UpdateRoleRequest) (*v1pb.Role, error) {
	principalID := ctx.Value(common.PrincipalIDContextKey).(int)
	roleID, err := getRoleID(request.Role.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}
	role, err := s.store.GetRole(ctx, roleID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get role: %v", err)
	}
	if role == nil {
		return nil, status.Errorf(codes.NotFound, "role not found: %s", roleID)
	}
	patch := &store.UpdateRoleMessage{
		UpdaterID:  principalID,
		ResourceID: roleID,
	}
	for _, path := range request.UpdateMask.Paths {
		switch path {
		case "description":
			patch.Description = &request.Role.Description
		default:
			return nil, status.Errorf(codes.InvalidArgument, "invalid update mask path: %s", path)
		}
	}

	roleMessage, err := s.store.UpdateRole(ctx, patch)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update role: %v", err)
	}
	return convertToRole(roleMessage), nil
}

// DeleteRole deletes an existing role.
func (s *RoleService) DeleteRole(ctx context.Context, request *v1pb.DeleteRoleRequest) (*emptypb.Empty, error) {
	roleID, err := getRoleID(request.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, err.Error())
	}
	role, err := s.store.GetRole(ctx, roleID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get role: %v", err)
	}
	if role == nil {
		return nil, status.Errorf(codes.NotFound, "role not found: %s", roleID)
	}
	if err := s.store.DeleteRole(ctx, roleID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete role: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func convertToRoles(roleMessages []*store.RoleMessage) []*v1pb.Role {
	var roles []*v1pb.Role
	for _, roleMessage := range roleMessages {
		roles = append(roles, convertToRole(roleMessage))
	}
	return roles
}

func convertToRole(role *store.RoleMessage) *v1pb.Role {
	return &v1pb.Role{
		Name:        convertToRoleName(role.ResourceID),
		Description: role.Description,
	}
}

func convertToRoleName(role string) string {
	return fmt.Sprintf("%s%s", rolePrefix, role)
}
