package rbac

func Authorize(roleStr string, actionStr string) bool {
	if roleStr == "" || actionStr == "" {
		return false
	}

	role := Role(roleStr)
	action := Permission(actionStr)

	permissions, ok := PolicyMatrix[role]
	if !ok {
		return false
	}

	for _, permission := range permissions {
		if permission == action {
			return true
		}
	}

	return false
}
