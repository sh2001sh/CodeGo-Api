package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	"testing"
)

func TestGetAllUsersReturnsPagedAdminList(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	users := []*identityschema.User{
		{Id: 1, Username: "user-one", Password: "password123", DisplayName: "User One", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"},
		{Id: 2, Username: "user-two", Password: "password123", DisplayName: "User Two", Role: constant.RoleAdminUser, Status: constant.UserStatusEnabled, Group: "default"},
	}
	for _, user := range users {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("failed to seed user: %v", err)
		}
	}

	ctx, recorder := newAuthenticatedContext(t, "GET", "/api/user/?p=1&page_size=10", nil, 100)
	GetAllUsers(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got %#v", response)
	}
}

func TestGetUserRejectsSameLevelAccess(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "admin-target", Password: "password123", DisplayName: "Admin Target", Role: constant.RoleAdminUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, "GET", "/api/user/1", nil, 99)
	ctx.Params = append(ctx.Params, gin.Param{Key: "id", Value: "1"})
	ctx.Set("role", constant.RoleAdminUser)
	GetUser(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected same-level access rejection, got %#v", response)
	}
}

func TestCreateUserCreatesAdminManagedUser(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	ctx, recorder := newAuthenticatedContext(t, "POST", "/api/user/", map[string]any{
		"username":     "created-user",
		"password":     "password123",
		"display_name": "Created User",
		"role":         constant.RoleCommonUser,
	}, 99)
	ctx.Set("role", constant.RoleAdminUser)
	CreateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected create success, got %#v", response)
	}

	reloaded, err := loadUserByIDForTest(1, true)
	if err != nil {
		t.Fatalf("failed to reload created user: %v", err)
	}
	if reloaded.Username != "created-user" {
		t.Fatalf("unexpected created user: %#v", reloaded)
	}
}

func TestUpdateUserEditsManagedUser(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "managed-user", Password: "password123", DisplayName: "Managed User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := db.Model(&identityschema.User{}).Where("id = ?", user.Id).Update("remark", "before").Error; err != nil {
		t.Fatalf("failed to seed remark: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, "PUT", "/api/user/", map[string]any{
		"id":           user.Id,
		"username":     user.Username,
		"display_name": "Updated Name",
		"group":        "vip",
		"remark":       "updated remark",
	}, 99)
	ctx.Set("role", constant.RoleAdminUser)
	UpdateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected update success, got %#v", response)
	}

	reloaded, err := loadUserByIDForTest(user.Id, false)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if reloaded.DisplayName != "Updated Name" || reloaded.Group != "vip" || reloaded.Remark != "updated remark" {
		t.Fatalf("unexpected updated user: %#v", reloaded)
	}
}

func TestDeleteUserHardDeletesManagedUser(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "delete-managed", Password: "password123", DisplayName: "Delete Managed", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, "DELETE", "/api/user/1", nil, 99)
	ctx.Params = append(ctx.Params, gin.Param{Key: "id", Value: "1"})
	ctx.Set("role", constant.RoleAdminUser)
	DeleteUser(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected delete success, got %#v", response)
	}
	if _, err := loadUserByIDForTest(user.Id, true); err == nil {
		t.Fatalf("expected hard-deleted user to be absent")
	}
}

func TestManageUserPromoteReturnsRoleStatusPayload(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "promote-user", Password: "password123", DisplayName: "Promote User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, "POST", "/api/user/manage", map[string]any{
		"id":     user.Id,
		"action": "promote",
	}, 99)
	ctx.Set("role", constant.RoleRootUser)
	ctx.Set("username", "root-admin")
	ManageUser(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected promote success, got %#v", response)
	}

	reloaded, err := loadUserByIDForTest(user.Id, false)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if reloaded.Role != constant.RoleAdminUser {
		t.Fatalf("expected promoted role, got %d", reloaded.Role)
	}
}

func TestManageUserOverrideQuotaUpdatesQuota(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "quota-user", Password: "password123", DisplayName: "Quota User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default", Quota: 100}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, "POST", "/api/user/manage", map[string]any{
		"id":     user.Id,
		"action": "add_quota",
		"mode":   "override",
		"value":  500,
	}, 99)
	ctx.Set("role", constant.RoleAdminUser)
	ctx.Set("username", "admin-user")
	ManageUser(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected quota override success, got %#v", response)
	}

	reloaded, err := loadUserByIDForTest(user.Id, false)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if reloaded.Quota != 500 {
		t.Fatalf("expected overridden quota, got %d", reloaded.Quota)
	}
}

func TestAdminClearUserBindingClearsField(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "binding-user", Password: "password123", DisplayName: "Binding User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default", GitHubId: "gh_123"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, "DELETE", "/api/user/1/bindings/github", nil, 99)
	ctx.Params = append(ctx.Params, gin.Param{Key: "id", Value: "1"})
	ctx.Params = append(ctx.Params, gin.Param{Key: "binding_type", Value: "github"})
	ctx.Set("role", constant.RoleAdminUser)
	AdminClearUserBinding(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected binding clear success, got %#v", response)
	}

	reloaded, err := loadUserByIDForTest(user.Id, false)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if reloaded.GitHubId != "" {
		t.Fatalf("expected github binding cleared, got %#v", reloaded)
	}
}
