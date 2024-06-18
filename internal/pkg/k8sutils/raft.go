package k8sutils

import (
	"fmt"
	"strings"
)

func ParseRaftNodesCount(replicas int32) (nodesCount int) {

	if replicas >= 10 {
		nodesCount = 5
	} else if replicas >= 3 {
		nodesCount = 3
	} else {
		nodesCount = 1
	}

	return nodesCount
}

func ParseRaftJoinNodes(dbName string, nodeCount int) string {
	var nodes []string

	for i := 0; i < nodeCount; i++ {
		nodes = append(nodes, fmt.Sprintf("%s-%d", dbName, i))
	}

	return strings.Join(nodes, ",")
}
