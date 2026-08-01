package auth

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleReadOnly Role = "read-only"
	RoleEditor   Role = "editor"
)

type Principal struct {
	Issuer   string
	Subject  string
	Username string
	Audience any
	Roles    []Role
	Claims   map[string]any
}

func (p Principal) HasRole(want Role) bool {
	for _, role := range p.Roles {
		if role == want {
			return true
		}
	}
	return false
}

func (p Principal) CanRead() bool {
	return p.HasRole(RoleAdmin) || p.HasRole(RoleReadOnly) || p.HasRole(RoleEditor)
}

func (p Principal) CanWrite() bool {
	return p.HasRole(RoleAdmin) || p.HasRole(RoleEditor)
}

func (p Principal) CanDelete() bool {
	return p.HasRole(RoleAdmin)
}

func DefaultAdminPrincipal() Principal {
	return Principal{
		Subject:  "admin",
		Username: "admin",
		Roles:    []Role{RoleAdmin},
	}
}
