package tf

import "k8s.io/apimachinery/pkg/runtime/schema"

// gvk is a tiny constructor to keep the registry table readable.
func gvk(group, version, kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: group, Version: version, Kind: kind}
}

// nativeRegistry maps well-known core/apps/batch GVKs to their native
// HashiCorp "kubernetes" provider resource type. Anything not in this table
// falls through to the kubernetes_manifest strategy (Strategy B).
var nativeRegistry = map[schema.GroupVersionKind]string{
	// core/v1
	gvk("", "v1", "Secret"):                "kubernetes_secret",
	gvk("", "v1", "Pod"):                   "kubernetes_pod",
	gvk("", "v1", "ConfigMap"):             "kubernetes_config_map",
	gvk("", "v1", "Service"):               "kubernetes_service",
	gvk("", "v1", "ServiceAccount"):        "kubernetes_service_account",
	gvk("", "v1", "Namespace"):             "kubernetes_namespace",
	gvk("", "v1", "PersistentVolumeClaim"): "kubernetes_persistent_volume_claim",
	gvk("", "v1", "PersistentVolume"):      "kubernetes_persistent_volume",
	gvk("", "v1", "ResourceQuota"):         "kubernetes_resource_quota",
	gvk("", "v1", "LimitRange"):            "kubernetes_limit_range",

	// apps/v1
	gvk("apps", "v1", "Deployment"):  "kubernetes_deployment",
	gvk("apps", "v1", "StatefulSet"): "kubernetes_stateful_set",
	gvk("apps", "v1", "DaemonSet"):   "kubernetes_daemon_set",
	gvk("apps", "v1", "ReplicaSet"):  "kubernetes_replica_set",

	// batch/v1
	gvk("batch", "v1", "Job"):     "kubernetes_job",
	gvk("batch", "v1", "CronJob"): "kubernetes_cron_job",

	// networking
	gvk("networking.k8s.io", "v1", "Ingress"):       "kubernetes_ingress_v1",
	gvk("networking.k8s.io", "v1", "NetworkPolicy"): "kubernetes_network_policy",

	// rbac
	gvk("rbac.authorization.k8s.io", "v1", "Role"):               "kubernetes_role",
	gvk("rbac.authorization.k8s.io", "v1", "RoleBinding"):        "kubernetes_role_binding",
	gvk("rbac.authorization.k8s.io", "v1", "ClusterRole"):        "kubernetes_cluster_role",
	gvk("rbac.authorization.k8s.io", "v1", "ClusterRoleBinding"): "kubernetes_cluster_role_binding",
}

// nativeType returns the native provider resource type for a GVK and whether a
// mapping exists.
func nativeType(g schema.GroupVersionKind) (string, bool) {
	t, ok := nativeRegistry[g]
	return t, ok
}

// IsNative reports whether the given group/version/kind has a native Terraform
// provider mapping (Strategy A). Used by the interactive picker to label types.
func IsNative(group, version, kind string) bool {
	_, ok := nativeRegistry[gvk(group, version, kind)]
	return ok
}

// NativeTypeFor returns the native provider resource type name for a
// group/version/kind, or "" if none.
func NativeTypeFor(group, version, kind string) string {
	return nativeRegistry[gvk(group, version, kind)]
}
