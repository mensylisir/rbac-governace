package kube

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	authnv1 "k8s.io/api/authentication/v1"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type Client struct {
	Config    *rest.Config
	Clientset kubernetes.Interface
	Dynamic   dynamic.Interface
	Discovery discovery.DiscoveryInterface
}

type AccessCheck struct {
	Allowed        bool
	Namespace      string
	Verb           string
	Group          string
	Resource       string
	Name           string
	Reason         string
	ServiceAccount string
}

type ObjectSnapshot struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
	YAML       string
	Exists     bool
}

type ServiceAccountToken struct {
	Token     string
	ExpiresAt time.Time
}

func DecodeYAML(src string) ([]unstructured.Unstructured, error) {
	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(src), 4096)
	out := []unstructured.Unstructured{}
	for {
		var obj unstructured.Unstructured
		err := dec.Decode(&obj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if obj.GetKind() == "" {
			continue
		}
		out = append(out, obj)
	}
	return out, nil
}

type ClusterInfo struct {
	APIServer string
	Context   string
}

type WorkloadRef struct {
	Type                   string
	Name                   string
	Namespace              string
	Kind                   string
	ServiceAccount         string
	Labels                 map[string]string
	RecommendedTemplateIDs []string
}

type ArgoCDStatus struct {
	Version                           string
	SyncImpersonation                 bool
	AppProjectsWithDSA                int
	ApplicationControllerClusterAdmin bool
}

type BindingRef struct {
	Kind      string
	Name      string
	Namespace string
	RoleKind  string
	RoleName  string
}

type SARule struct {
	Binding BindingRef
	Rule    rbacv1.PolicyRule
}

func NewFromKubeconfig(kubeconfig string) (*Client, ClusterInfo, error) {
	loader, err := clientcmd.Load([]byte(kubeconfig))
	if err != nil {
		return nil, ClusterInfo{}, fmt.Errorf("parse kubeconfig: %w", err)
	}
	overrides := &clientcmd.ConfigOverrides{}
	config, err := clientcmd.NewDefaultClientConfig(*loader, overrides).ClientConfig()
	if err != nil {
		return nil, ClusterInfo{}, fmt.Errorf("build kube config: %w", err)
	}
	info := ClusterInfo{APIServer: config.Host, Context: currentContext(loader)}
	client, err := newClient(config, info)
	if err != nil {
		return nil, ClusterInfo{}, err
	}
	return client, info, nil
}

func NewInCluster() (*Client, ClusterInfo, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, ClusterInfo{}, fmt.Errorf("build in-cluster config: %w", err)
	}
	info := ClusterInfo{APIServer: config.Host, Context: "in-cluster"}
	client, err := newClient(config, info)
	if err != nil {
		return nil, ClusterInfo{}, err
	}
	return client, info, nil
}

func newClient(config *rest.Config, info ClusterInfo) (*Client, error) {
	config.Timeout = 20 * time.Second
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}
	return &Client{
		Config:    config,
		Clientset: cs,
		Dynamic:   dyn,
		Discovery: cs.Discovery(),
	}, nil
}

func currentContext(cfg *api.Config) string {
	if cfg.CurrentContext != "" {
		return cfg.CurrentContext
	}
	for name := range cfg.Contexts {
		return name
	}
	return ""
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Clientset.Discovery().ServerVersion()
	return err
}

func (c *Client) ServerCAData() []byte {
	if len(c.Config.CAData) > 0 {
		return append([]byte(nil), c.Config.CAData...)
	}
	return nil
}

func (c *Client) CreateServiceAccountToken(ctx context.Context, namespace, serviceAccount string, expirationSeconds int64) (ServiceAccountToken, error) {
	if expirationSeconds <= 0 {
		expirationSeconds = int64((8 * time.Hour).Seconds())
	}
	req := &authnv1.TokenRequest{
		Spec: authnv1.TokenRequestSpec{
			ExpirationSeconds: &expirationSeconds,
		},
	}
	token, err := c.Clientset.CoreV1().ServiceAccounts(namespace).CreateToken(ctx, serviceAccount, req, metav1.CreateOptions{})
	if err != nil {
		return ServiceAccountToken{}, err
	}
	return ServiceAccountToken{Token: token.Status.Token, ExpiresAt: token.Status.ExpirationTimestamp.Time}, nil
}

func (c *Client) HasRBACManager(ctx context.Context) (bool, error) {
	for _, gv := range []string{"rbacmanager.dev/v1beta1", "rbacmanager.reactiveops.io/v1beta1", "rbac-manager.reactiveops.io/v1beta1"} {
		resources, err := c.Discovery.ServerResourcesForGroupVersion(gv)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				continue
			}
			continue
		}
		for _, r := range resources.APIResources {
			if r.Kind == "RBACDefinition" {
				return true, nil
			}
		}
	}
	return false, nil
}

func (c *Client) DiscoverWorkloads(ctx context.Context) ([]WorkloadRef, error) {
	deployments, err := c.Clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	statefulsets, err := c.Clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	daemonsets, err := c.Clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	out := []WorkloadRef{}
	for _, d := range deployments.Items {
		out = append(out, workloadFromDeployment(classify(d.Name, d.Namespace, d.Labels), d))
	}
	for _, s := range statefulsets.Items {
		out = append(out, workloadFromStatefulSet(classify(s.Name, s.Namespace, s.Labels), s))
	}
	for _, d := range daemonsets.Items {
		out = append(out, workloadFromDaemonSet(classify(d.Name, d.Namespace, d.Labels), d))
	}
	return out, nil
}

func (c *Client) ArgoCDStatus(ctx context.Context, namespace, controllerServiceAccount string) ArgoCDStatus {
	status := ArgoCDStatus{}
	cm, err := c.Clientset.CoreV1().ConfigMaps(namespace).Get(ctx, "argocd-cm", metav1.GetOptions{})
	if err == nil {
		status.SyncImpersonation = cm.Data["application.sync.impersonation.enabled"] == "true"
	}
	deploy, err := c.Clientset.AppsV1().Deployments(namespace).Get(ctx, "argocd-server", metav1.GetOptions{})
	if err == nil {
		for _, ctn := range deploy.Spec.Template.Spec.Containers {
			if ctn.Name == "argocd-server" || strings.Contains(ctn.Image, "argocd") {
				parts := strings.Split(ctn.Image, ":")
				if len(parts) > 1 {
					status.Version = parts[len(parts)-1]
				}
			}
		}
	}
	projects := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "appprojects"}
	list, err := c.Dynamic.Resource(projects).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range list.Items {
			dsa, ok, _ := unstructured.NestedSlice(item.Object, "spec", "destinationServiceAccounts")
			if ok && len(dsa) > 0 {
				status.AppProjectsWithDSA++
			}
		}
	}
	if controllerServiceAccount != "" {
		rules, err := c.RulesForServiceAccount(ctx, namespace, controllerServiceAccount)
		if err == nil {
			for _, rule := range rules {
				if rule.Binding.RoleKind == "ClusterRole" && rule.Binding.RoleName == "cluster-admin" {
					status.ApplicationControllerClusterAdmin = true
					break
				}
			}
		}
	}
	return status
}

func workloadFromDeployment(toolType string, d appsv1.Deployment) WorkloadRef {
	return WorkloadRef{Type: toolType, Name: d.Name, Namespace: d.Namespace, Kind: "Deployment", ServiceAccount: serviceAccountName(d.Spec.Template.Spec.ServiceAccountName), Labels: d.Labels}
}

func workloadFromStatefulSet(toolType string, s appsv1.StatefulSet) WorkloadRef {
	return WorkloadRef{Type: toolType, Name: s.Name, Namespace: s.Namespace, Kind: "StatefulSet", ServiceAccount: serviceAccountName(s.Spec.Template.Spec.ServiceAccountName), Labels: s.Labels}
}

func workloadFromDaemonSet(toolType string, d appsv1.DaemonSet) WorkloadRef {
	return WorkloadRef{Type: toolType, Name: d.Name, Namespace: d.Namespace, Kind: "DaemonSet", ServiceAccount: serviceAccountName(d.Spec.Template.Spec.ServiceAccountName), Labels: d.Labels}
}

func serviceAccountName(name string) string {
	if name == "" {
		return "default"
	}
	return name
}

func classify(name, namespace string, labels map[string]string) string {
	text := strings.ToLower(name + " " + namespace + " " + labels["app.kubernetes.io/name"] + " " + labels["app"] + " " + labels["app.kubernetes.io/instance"])
	switch {
	case strings.Contains(text, "argocd"):
		return "argocd"
	case strings.Contains(text, "jenkins"):
		return "jenkins"
	case strings.Contains(text, "prometheus"):
		return "prometheus"
	case strings.Contains(text, "loki"):
		return "loki"
	case strings.Contains(text, "promtail") || strings.Contains(text, "grafana-agent") || strings.Contains(text, "alloy"):
		return "log-collector"
	default:
		return "custom"
	}
}

func (c *Client) RulesForServiceAccount(ctx context.Context, namespace, name string) ([]SARule, error) {
	roleBindings, err := c.Clientset.RbacV1().RoleBindings("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	clusterRoleBindings, err := c.Clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	out := []SARule{}
	for _, rb := range roleBindings.Items {
		if !hasServiceAccountSubject(rb.Subjects, namespace, name) {
			continue
		}
		rules, err := c.rulesForRoleRef(ctx, rb.Namespace, rb.RoleRef)
		if err != nil {
			continue
		}
		for _, rule := range rules {
			out = append(out, SARule{Binding: BindingRef{Kind: "RoleBinding", Name: rb.Name, Namespace: rb.Namespace, RoleKind: rb.RoleRef.Kind, RoleName: rb.RoleRef.Name}, Rule: rule})
		}
	}
	for _, crb := range clusterRoleBindings.Items {
		if !hasServiceAccountSubject(crb.Subjects, namespace, name) {
			continue
		}
		rules, err := c.rulesForRoleRef(ctx, "", crb.RoleRef)
		if err != nil {
			continue
		}
		for _, rule := range rules {
			out = append(out, SARule{Binding: BindingRef{Kind: "ClusterRoleBinding", Name: crb.Name, RoleKind: crb.RoleRef.Kind, RoleName: crb.RoleRef.Name}, Rule: rule})
		}
	}
	return out, nil
}

func hasServiceAccountSubject(subjects []rbacv1.Subject, namespace, name string) bool {
	for _, s := range subjects {
		if s.Kind == "ServiceAccount" && s.Name == name && s.Namespace == namespace {
			return true
		}
	}
	return false
}

func (c *Client) rulesForRoleRef(ctx context.Context, namespace string, ref rbacv1.RoleRef) ([]rbacv1.PolicyRule, error) {
	if ref.Kind == "ClusterRole" {
		cr, err := c.Clientset.RbacV1().ClusterRoles().Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return cr.Rules, nil
	}
	role, err := c.Clientset.RbacV1().Roles(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return role.Rules, nil
}

func (c *Client) ApplyYAML(ctx context.Context, docs []unstructured.Unstructured) error {
	for i := range docs {
		obj := &docs[i]
		gvr, namespaced, err := c.gvrFor(obj.GroupVersionKind())
		if err != nil {
			return err
		}
		if namespaced {
			ns := obj.GetNamespace()
			if ns == "" {
				return fmt.Errorf("%s/%s requires namespace", obj.GetKind(), obj.GetName())
			}
			_, err = c.Dynamic.Resource(gvr).Namespace(ns).Apply(ctx, obj.GetName(), obj, metav1.ApplyOptions{FieldManager: "rbac-governance-console", Force: true})
		} else {
			_, err = c.Dynamic.Resource(gvr).Apply(ctx, obj.GetName(), obj, metav1.ApplyOptions{FieldManager: "rbac-governance-console", Force: true})
		}
		if err != nil {
			return fmt.Errorf("apply %s/%s: %w", obj.GetKind(), obj.GetName(), err)
		}
	}
	return nil
}

func (c *Client) ValidateServiceAccount(ctx context.Context, namespace, serviceAccount string, checks []AccessCheck) ([]AccessCheck, error) {
	out := make([]AccessCheck, 0, len(checks))
	user := "system:serviceaccount:" + namespace + ":" + serviceAccount
	for _, check := range checks {
		sar := &authv1.SubjectAccessReview{
			Spec: authv1.SubjectAccessReviewSpec{
				User: user,
				ResourceAttributes: &authv1.ResourceAttributes{
					Namespace: check.Namespace,
					Verb:      check.Verb,
					Group:     check.Group,
					Resource:  check.Resource,
					Name:      check.Name,
				},
			},
		}
		result, err := c.Clientset.AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
		check.Allowed = result.Status.Allowed
		check.Reason = result.Status.Reason
		check.ServiceAccount = namespace + "/" + serviceAccount
		out = append(out, check)
	}
	return out, nil
}

func (c *Client) SnapshotObjects(ctx context.Context, docs []unstructured.Unstructured) ([]ObjectSnapshot, error) {
	out := []ObjectSnapshot{}
	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)
	for i := range docs {
		obj := &docs[i]
		gvr, namespaced, err := c.gvrFor(obj.GroupVersionKind())
		if err != nil {
			return nil, err
		}
		var existing *unstructured.Unstructured
		if namespaced {
			existing, err = c.Dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Get(ctx, obj.GetName(), metav1.GetOptions{})
		} else {
			existing, err = c.Dynamic.Resource(gvr).Get(ctx, obj.GetName(), metav1.GetOptions{})
		}
		snapshot := ObjectSnapshot{APIVersion: obj.GetAPIVersion(), Kind: obj.GetKind(), Namespace: obj.GetNamespace(), Name: obj.GetName()}
		if errors.IsNotFound(err) {
			snapshot.Exists = false
			out = append(out, snapshot)
			continue
		}
		if err != nil {
			return nil, err
		}
		var b strings.Builder
		if err := serializer.Encode(existing, &b); err != nil {
			return nil, err
		}
		snapshot.Exists = true
		snapshot.YAML = b.String()
		out = append(out, snapshot)
	}
	return out, nil
}

func (c *Client) SnapshotResources(ctx context.Context, refs []ObjectSnapshot) ([]ObjectSnapshot, error) {
	out := []ObjectSnapshot{}
	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)
	for _, ref := range refs {
		gvk := schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind)
		gvr, namespaced, err := c.gvrFor(gvk)
		if err != nil {
			return nil, err
		}
		var existing *unstructured.Unstructured
		if namespaced {
			existing, err = c.Dynamic.Resource(gvr).Namespace(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		} else {
			existing, err = c.Dynamic.Resource(gvr).Get(ctx, ref.Name, metav1.GetOptions{})
		}
		snapshot := ObjectSnapshot{APIVersion: ref.APIVersion, Kind: ref.Kind, Namespace: ref.Namespace, Name: ref.Name}
		if errors.IsNotFound(err) {
			snapshot.Exists = false
			out = append(out, snapshot)
			continue
		}
		if err != nil {
			return nil, err
		}
		var b strings.Builder
		if err := serializer.Encode(existing, &b); err != nil {
			return nil, err
		}
		snapshot.Exists = true
		snapshot.YAML = b.String()
		out = append(out, snapshot)
	}
	return out, nil
}

func (c *Client) DeleteResources(ctx context.Context, refs []ObjectSnapshot) error {
	for _, ref := range refs {
		gvk := schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind)
		gvr, namespaced, err := c.gvrFor(gvk)
		if err != nil {
			return err
		}
		if namespaced {
			err = c.Dynamic.Resource(gvr).Namespace(ref.Namespace).Delete(ctx, ref.Name, metav1.DeleteOptions{})
		} else {
			err = c.Dynamic.Resource(gvr).Delete(ctx, ref.Name, metav1.DeleteOptions{})
		}
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete %s/%s: %w", ref.Kind, ref.Name, err)
		}
	}
	return nil
}

func (c *Client) RestoreSnapshots(ctx context.Context, snapshots []ObjectSnapshot) error {
	for _, snapshot := range snapshots {
		gvk := schema.FromAPIVersionAndKind(snapshot.APIVersion, snapshot.Kind)
		gvr, namespaced, err := c.gvrFor(gvk)
		if err != nil {
			return err
		}
		if !snapshot.Exists {
			if namespaced {
				err = c.Dynamic.Resource(gvr).Namespace(snapshot.Namespace).Delete(ctx, snapshot.Name, metav1.DeleteOptions{})
			} else {
				err = c.Dynamic.Resource(gvr).Delete(ctx, snapshot.Name, metav1.DeleteOptions{})
			}
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			continue
		}
		objects, err := DecodeYAML(snapshot.YAML)
		if err != nil {
			return err
		}
		for i := range objects {
			unstructured.RemoveNestedField(objects[i].Object, "metadata", "managedFields")
			unstructured.RemoveNestedField(objects[i].Object, "metadata", "resourceVersion")
			unstructured.RemoveNestedField(objects[i].Object, "metadata", "creationTimestamp")
			unstructured.RemoveNestedField(objects[i].Object, "metadata", "uid")
			unstructured.RemoveNestedField(objects[i].Object, "status")
		}
		if err := c.ApplyYAML(ctx, objects); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) gvrFor(gvk schema.GroupVersionKind) (schema.GroupVersionResource, bool, error) {
	resources, err := c.Discovery.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return schema.GroupVersionResource{}, false, err
	}
	for _, r := range resources.APIResources {
		if r.Kind == gvk.Kind {
			return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: r.Name}, r.Namespaced, nil
		}
	}
	return schema.GroupVersionResource{}, false, fmt.Errorf("resource for %s not found", gvk.String())
}

func PodSecurityFindings(pod corev1.PodSpec) []string {
	out := []string{}
	for _, v := range pod.Volumes {
		if v.HostPath != nil {
			out = append(out, "uses hostPath volume "+v.Name)
		}
	}
	for _, c := range pod.Containers {
		if c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
			out = append(out, "container "+c.Name+" is privileged")
		}
	}
	return out
}
