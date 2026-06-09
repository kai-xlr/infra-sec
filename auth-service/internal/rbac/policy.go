package rbac

type Permission string

const (
	PermissionRead   Permission = "read"
	PermissionWrite  Permission = "write"
	PermissionDelete Permission = "delete"
)

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
	RoleViewer    Role = "viewer"
)

var PolicyMatrix = map[Role][]Permission{
	RoleAdmin: {
		PermissionRead,
		PermissionWrite,
		PermissionDelete,
	},
	RoleDeveloper: {
		PermissionRead,
		PermissionWrite,
	},
	RoleViewer: {
		PermissionRead,
	},
}
