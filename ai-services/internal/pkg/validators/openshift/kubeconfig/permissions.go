package kubeconfig

const (
	operatorGroup = "operators.coreos.com"
	coreGroup     = ""
	rbacGroup     = "rbac.authorization.k8s.io"
	sccGroup      = "security.openshift.io"
)

// permissionCheck defines a permission that needs to be validated.
type permissionCheck struct {
	group     string
	resource  string
	verb      string
	namespace string // empty string means cluster-scoped
}

// getBootstrapWildcardPermissions returns wildcard permission checks for bootstrap operations.
// Focuses on core operator installation permissions - operators manage their own custom resources.
func getBootstrapWildcardPermissions() []permissionCheck {
	return []permissionCheck{
		// Check for all verbs on all resources in operators.coreos.com group (covers operatorgroups, subscriptions, csvs)
		{group: operatorGroup, resource: "*", verb: "*", namespace: ""},

		// Check for all verbs on all resources in core group (covers namespaces)
		{group: coreGroup, resource: "*", verb: "*", namespace: ""},
	}
}

// getApplicationCreateWildcardPermissions returns wildcard permission checks for application creation.
// Using wildcards (*) to check for admin-level permissions needed for deploying applications.
func getApplicationCreateWildcardPermissions() []permissionCheck {
	return []permissionCheck{
		// Check for all verbs on all resources in security.openshift.io group (covers SCCs)
		{group: sccGroup, resource: "*", verb: "*", namespace: ""},

		// Check for all verbs on all resources in rbac.authorization.k8s.io group (covers roles, rolebindings)
		{group: rbacGroup, resource: "*", verb: "*", namespace: ""},
	}
}

// getBootstrapSpecificPermissions returns detailed permission checks for bootstrap operations.
// Focuses on core operator installation permissions - if user can install operators,
// they typically have permissions for the custom resources those operators manage.
func getBootstrapSpecificPermissions() []permissionCheck {
	return []permissionCheck{
		// Namespace operations (cluster-scoped)
		{group: coreGroup, resource: "namespaces", verb: "create", namespace: ""},
		{group: coreGroup, resource: "namespaces", verb: "get", namespace: ""},
		{group: coreGroup, resource: "namespaces", verb: "patch", namespace: ""},

		// OperatorGroup operations (cluster-wide)
		{group: operatorGroup, resource: "operatorgroups", verb: "create", namespace: ""},
		{group: operatorGroup, resource: "operatorgroups", verb: "get", namespace: ""},
		{group: operatorGroup, resource: "operatorgroups", verb: "patch", namespace: ""},

		// Subscription operations (cluster-wide)
		{group: operatorGroup, resource: "subscriptions", verb: "create", namespace: ""},
		{group: operatorGroup, resource: "subscriptions", verb: "get", namespace: ""},
		{group: operatorGroup, resource: "subscriptions", verb: "patch", namespace: ""},

		// ClusterServiceVersion operations (cluster-wide, needed to check operator status)
		{group: operatorGroup, resource: "clusterserviceversions", verb: "get", namespace: ""},
	}
}

// getApplicationCreateSpecificPermissions returns detailed permission checks for application creation.
// These are the exact permissions needed for deploying applications with SCC bindings.
func getApplicationCreateSpecificPermissions() []permissionCheck {
	return []permissionCheck{
		// SecurityContextConstraints "use" permission (needed to grant SCC usage in Roles)
		{group: sccGroup, resource: "securitycontextconstraints", verb: "use", namespace: ""},

		// Role operations (needed to create Roles that grant SCC usage)
		{group: rbacGroup, resource: "roles", verb: "create", namespace: ""},
		{group: rbacGroup, resource: "roles", verb: "get", namespace: ""},
		{group: rbacGroup, resource: "roles", verb: "patch", namespace: ""},

		// RoleBinding operations (needed to bind Roles to ServiceAccounts, assigning SCC to them)
		{group: rbacGroup, resource: "rolebindings", verb: "create", namespace: ""},
		{group: rbacGroup, resource: "rolebindings", verb: "get", namespace: ""},
		{group: rbacGroup, resource: "rolebindings", verb: "update", namespace: ""},
		{group: rbacGroup, resource: "rolebindings", verb: "patch", namespace: ""},
	}
}

// getWildcardPermissions returns all wildcard permission checks (bootstrap + application create).
func getWildcardPermissions() []permissionCheck {
	perms := getBootstrapWildcardPermissions()
	perms = append(perms, getApplicationCreateWildcardPermissions()...)

	return perms
}

// getSpecificPermissions returns all specific permission checks (bootstrap + application create).
func getSpecificPermissions() []permissionCheck {
	perms := getBootstrapSpecificPermissions()
	perms = append(perms, getApplicationCreateSpecificPermissions()...)

	return perms
}

// Made with Bob
