package k8sutils

import (
	"fmt"
	"github.com/brodyaga777/weaviate-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func ParseEnvs(db v1alpha1.Weaviate) []corev1.EnvVar {

	var envs []corev1.EnvVar

	// standalone mode always
	envs = append(envs, corev1.EnvVar{
		Name:  "CLUSTER_DATA_BIND_PORT",
		Value: "7001",
	})
	envs = append(envs, corev1.EnvVar{
		Name:  "CLUSTER_GOSSIP_BIND_PORT",
		Value: "7000",
	})
	envs = append(envs, corev1.EnvVar{
		Name:  "GOGC",
		Value: "100",
	})

	// TODO: if monitoring.enabled
	envs = append(envs, corev1.EnvVar{
		Name:  "PROMETHEUS_MONITORING_ENABLED",
		Value: "false",
	})
	envs = append(envs, corev1.EnvVar{
		Name:  "PROMETHEUS_MONITORING_GROUP",
		Value: "false",
	})

	envs = append(envs, corev1.EnvVar{
		Name:  "QUERY_MAXIMUM_RESULTS",
		Value: "100000",
	})
	envs = append(envs, corev1.EnvVar{
		Name:  "REINDEX_VECTOR_DIMENSIONS_AT_STARTUP",
		Value: "false",
	})
	envs = append(envs, corev1.EnvVar{
		Name:  "TRACK_VECTOR_DIMENSIONS",
		Value: "false",
	})

	// auth
	envs = append(envs, corev1.EnvVar{
		Name: "CLUSTER_BASIC_AUTH_USERNAME",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: fmt.Sprintf("%s-cluster-api-basic-auth", db.Name)},
				Key: "username",
			},
		},
	})

	envs = append(envs, corev1.EnvVar{
		Name: "CLUSTER_BASIC_AUTH_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: fmt.Sprintf("%s-cluster-api-basic-auth", db.Name)},
				Key: "password",
			},
		},
	})

	envs = append(envs, corev1.EnvVar{
		Name:  "STANDALONE_MODE",
		Value: "true",
	})

	// TODO: support pvc, dynamic persistence
	envs = append(envs, corev1.EnvVar{
		Name:  "PERSISTENCE_DATA_PATH",
		Value: "/var/lib/weaviate",
	})

	envs = append(envs, corev1.EnvVar{
		Name:  "DEFAULT_VECTORIZER_MODULE",
		Value: "none",
	})

	nodesCount := ParseRaftNodesCount(db.Spec.Replicas)
	raftJoinNodes := ParseRaftJoinNodes(db.Name, nodesCount)

	envs = append(envs, corev1.EnvVar{
		Name:  "RAFT_JOIN",
		Value: raftJoinNodes,
	})

	envs = append(envs, corev1.EnvVar{
		Name:  "RAFT_BOOTSTRAP_EXPECT",
		Value: fmt.Sprintf("%d", nodesCount),
	})

	envs = append(envs, corev1.EnvVar{
		Name:  "CLUSTER_JOIN",
		Value: fmt.Sprintf("%s-headless.%s.svc.cluster.local.", db.Name, db.Namespace),
	})
	envs = append(envs, corev1.EnvVar{
		Name:  "DISABLE_TELEMETRY",
		Value: "true",
	})
	envs = append(envs, corev1.EnvVar{
		Name:  "LOG_LEVEL",
		Value: "debug",
	})
	envs = append(envs, corev1.EnvVar{
		Name:  "AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED",
		Value: "true",
	})
	envs = append(envs, corev1.EnvVar{
		Name:  "AUTHENTICATION_OIDC_ENABLED",
		Value: "false",
	})
	envs = append(envs, corev1.EnvVar{
		Name:  "AUTHORIZATION_ADMINLIST_ENABLED",
		Value: "false",
	})
	envs = append(envs, corev1.EnvVar{
		Name:  "QUERY_DEFAULTS_LIMIT",
		Value: "100",
	})

	return envs
}
