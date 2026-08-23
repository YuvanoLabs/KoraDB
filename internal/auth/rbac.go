package auth

import "strings"

// methodMinRole maps each gRPC method (short name) to the minimum role required
// to call it. This map is the authorization policy. It is consulted
// fail-closed: any method NOT listed here is denied, so adding a new RPC and
// forgetting to classify it results in "denied to everyone", never "open to
// everyone".
var methodMinRole = map[string]Role{
	// Reads.
	"Get":             RoleReadOnly,
	"Query":           RoleReadOnly,
	"ListSchemas":     RoleReadOnly,
	"ListCollections": RoleReadOnly,
	// Writes.
	"Insert": RoleReadWrite,
	"Update": RoleReadWrite,
	"Delete": RoleReadWrite,
	// Administrative (DDL + key management).
	"PutSchema":        RoleAdmin,
	"CreateCollection": RoleAdmin,
	"CreateKey":        RoleAdmin,
	"ListKeys":         RoleAdmin,
	"RevokeKey":        RoleAdmin,
}

// RequiredRole returns the minimum role for a full gRPC method name such as
// "/KoraDB.v1.KoraDB/Insert". ok is false for unmapped methods (caller must
// treat that as denied).
func RequiredRole(fullMethod string) (Role, bool) {
	name := fullMethod
	if i := strings.LastIndex(fullMethod, "/"); i >= 0 {
		name = fullMethod[i+1:]
	}
	r, ok := methodMinRole[name]
	return r, ok
}

// Can reports whether the principal may invoke fullMethod. Fail-closed: unknown
// methods and a nil principal are always denied.
func (p *Principal) Can(fullMethod string) bool {
	if p == nil {
		return false
	}
	need, ok := RequiredRole(fullMethod)
	if !ok {
		return false
	}
	return p.Role >= need
}
