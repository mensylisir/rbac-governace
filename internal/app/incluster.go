package app

import (
	"context"
	"os"
	"time"

	"rbac-manager/internal/kube"
)

func (s *Server) autoRegisterInCluster() {
	if os.Getenv("AUTO_IN_CLUSTER") == "false" {
		return
	}
	client, info, err := kube.NewInCluster()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		return
	}
	for _, c := range s.store.ListClusters() {
		if c.Kubeconfig == "IN_CLUSTER" {
			return
		}
	}
	hasRBACManager, _ := client.HasRBACManager(ctx)
	cluster := s.store.PutCluster(Cluster{
		Name: "in-cluster", Context: info.Context, Kubeconfig: "IN_CLUSTER", APIServer: info.APIServer,
		Status: "connected", Message: "auto-detected in-cluster connection", RBACDefinitionFound: hasRBACManager, RBACManagerStatus: rbacStatus(hasRBACManager),
	})
	s.audit("cluster.auto_in_cluster", cluster.ID, "", "", "success", cluster.Message)
}
