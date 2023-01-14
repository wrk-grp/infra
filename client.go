package infra

import (
	"context"
	"fmt"

	clientset "github.com/minio/operator/pkg/client/clientset/versioned"
	helmclient "github.com/mittwald/go-helm-client"
	promclientset "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/pytimer/k8sutil/apply"
	"github.com/theapemachine/wrkspc/brazil"
	"github.com/theapemachine/wrkspc/tweaker"
	"github.com/wrk-grp/errnie"
	"helm.sh/helm/v3/pkg/repo"
	v1 "k8s.io/api/core/v1"
	apiextension "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

/*
Client wraps the various Kubernetes clients that are needed to manipulate both
Kubernetes native, as well as extended or custom resources.
*/
type Client struct {
	KubeClient       *kubernetes.Clientset
	dynamicClient    dynamic.Interface
	discoveryClient  *discovery.DiscoveryClient
	ControllerClient *clientset.Clientset
	ExtClient        *apiextension.Clientset
	PromClient       *promclientset.Clientset
	HelmClient       helmclient.Client
}

/*
NewClient returns a handle on the various clients that we will need access to.
*/
func NewClient() *Client {
	config, err := clientcmd.BuildConfigFromFlags(
		"", brazil.NewPath("~/.kube/config").Location,
	)
	errnie.Handles(err)

	kubeClient, err := kubernetes.NewForConfig(config)
	errnie.Handles(err)

	dynamicClient, err := dynamic.NewForConfig(config)
	errnie.Handles(err)

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	errnie.Handles(err)

	controllerClient, err := clientset.NewForConfig(config)
	errnie.Handles(err)

	extClient, err := apiextension.NewForConfig(config)
	errnie.Handles(err)

	promClient, err := promclientset.NewForConfig(config)
	errnie.Handles(err)

	hc, err := helmclient.New(&helmclient.Options{
		Debug: false, Output: errnie.Ctx(),
	})
	errnie.Handles(err)

	return &Client{
		KubeClient:       kubeClient,
		dynamicClient:    dynamicClient,
		discoveryClient:  discoveryClient,
		ControllerClient: controllerClient,
		ExtClient:        extClient,
		PromClient:       promClient,
		HelmClient:       hc,
	}
}

func (client Client) Apply(scope string, values map[string]string) {
	if values["type"] == "helm" {
		errnie.Informs(fmt.Sprintf(
			"%s applying %s from %s to %s",
			values["type"], values["name"], values["url"], values["namespace"],
		))

		client.helm(scope, values)
		return
	}

	// We have a standard Kubernetes manifest and can just apply it.
	applyOpts := apply.NewApplyOptions(
		client.dynamicClient, client.discoveryClient,
	)

	// Create the namespace.
	if scope == "services" {
		client.KubeClient.CoreV1().Namespaces().Create(context.TODO(),
			&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tweaker.Stage()}},
			metav1.CreateOptions{},
		)
		for _, f := range []string{
			"configmap", "autoscaler", "deployment", "service",
		} {
			errnie.Handles(applyOpts.Apply(context.TODO(),
				NewManifest(scope, values, f).Compile(),
			))
		}

		return
	}

	// Read the manifest and apply any customizations needed to be
	// specific for a particular service.
	errnie.Handles(applyOpts.Apply(context.TODO(),
		NewManifest(scope, values, "").Compile(),
	))
}

func (client Client) helm(scope string, values map[string]string) {
	// Add the chart repository to the client.
	if values["repo"] != "" && values["url"] != "" {
		errnie.Handles(client.HelmClient.AddOrUpdateChartRepo(
			repo.Entry{
				Name: values["repo"],
				URL:  values["url"],
			},
		))
	}

	var err error
	var data []byte

	if values["values"] != "" {
		if data, err = embedded.ReadFile(
			fmt.Sprintf("cfg/%s/%s", scope, values["values"]),
		); errnie.Handles(err) != nil {
			return
		}
	}

	chartSpec := helmclient.ChartSpec{
		ReleaseName:     values["name"],
		ChartName:       values["chart"],
		Namespace:       values["namespace"],
		ValuesYaml:      string(data),
		CreateNamespace: true,
		UpgradeCRDs:     true,
		CleanupOnFail:   true,
	}

	_, err = client.HelmClient.InstallOrUpgradeChart(
		context.Background(), &chartSpec, nil,
	)

	errnie.Handles(err)
}
