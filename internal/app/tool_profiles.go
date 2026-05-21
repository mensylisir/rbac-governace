package app

import (
	"strings"

	"rbac-manager/internal/kube"
)

func builtinToolProfiles() []ToolProfile {
	return []ToolProfile{
		{ID: "builtin-argocd", Type: "argocd", Name: "Argo CD", MatchText: "argocd", RecommendedTemplateIDs: []string{"argocd-control-plane"}, Builtin: true},
		{ID: "builtin-jenkins", Type: "jenkins", Name: "Jenkins", MatchText: "jenkins", RecommendedTemplateIDs: []string{"jenkins-agent-manager", "jenkins-namespace-edit"}, Builtin: true},
		{ID: "builtin-prometheus", Type: "prometheus", Name: "Prometheus", MatchText: "prometheus", RecommendedTemplateIDs: []string{"prometheus-cluster-reader", "prometheus-namespace-reader"}, Builtin: true},
		{ID: "builtin-loki", Type: "loki", Name: "Loki", MatchText: "loki", RecommendedTemplateIDs: []string{"loki-namespace-reader"}, Builtin: true},
		{ID: "builtin-log-collector", Type: "log-collector", Name: "Log Collector", MatchText: "promtail,grafana-agent,alloy", RecommendedTemplateIDs: []string{"promtail-cluster-metadata-reader"}, Builtin: true},
	}
}

func matchToolProfile(ref kube.WorkloadRef, profiles []ToolProfile) (string, []string) {
	text := strings.ToLower(ref.Name + " " + ref.Namespace + " " + ref.Labels["app.kubernetes.io/name"] + " " + ref.Labels["app"] + " " + ref.Labels["app.kubernetes.io/instance"])
	for _, profile := range profiles {
		if labelsMatch(ref.Labels, profile.Labels) && textMatches(text, profile.MatchText) {
			return profile.Type, profile.RecommendedTemplateIDs
		}
	}
	return "", nil
}

func isArgoApplicationController(ref kube.WorkloadRef) bool {
	if ref.Type != "argocd" {
		return false
	}
	component := strings.ToLower(ref.Labels["app.kubernetes.io/component"])
	name := strings.ToLower(ref.Name)
	labelName := strings.ToLower(ref.Labels["app.kubernetes.io/name"])
	return component == "application-controller" || strings.Contains(name, "application-controller") || strings.Contains(labelName, "application-controller")
}

func labelsMatch(labels map[string]string, selector map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func textMatches(text, matchText string) bool {
	matchText = strings.TrimSpace(strings.ToLower(matchText))
	if matchText == "" {
		return true
	}
	for _, token := range strings.Split(matchText, ",") {
		token = strings.TrimSpace(token)
		if token != "" && strings.Contains(text, token) {
			return true
		}
	}
	return false
}
