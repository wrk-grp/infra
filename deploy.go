package infra

import (
	"embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/theapemachine/wrkspc/brazil"
	"github.com/theapemachine/wrkspc/tweaker"
	"github.com/wrk-grp/errnie"
)

//go:embed cfg/* cfg/infrastructure/* cfg/services/*
var embedded embed.FS

/*
Deploy wraps the process of reading the embedded manifest
configuration, which orchestrates the provisioning of
the platform and infrastructure.
*/
type Deploy struct {
	cluster *Kind
	cfg     *viper.Viper
}

/*
NewDeploy constructs an instance of Deploy and returns
a pointer reference to it.
*/
func NewDeploy(cluster *Kind) *Deploy {
	// Use viper to read in the provisioning configuration
	// file, and store a reference for later.
	v := viper.New()
	v.AddConfigPath(brazil.NewPath("~/.manifests.yml").Location)
	v.SetConfigType("yml")
	v.SetConfigName(".manifests")
	errnie.Handles(v.ReadInConfig())

	return &Deploy{cluster, v}
}

/*
Do the deployment, which brings up everything we need in the cluster.
*/
func (deploy *Deploy) Do() error {
	// Loop over the two root sections of the provisioning config.
	for _, scope := range []string{"infrastructure", "services"} {
		errnie.Informs(fmt.Sprintf("deploying %s", scope))

		// Retrieve the key/value pairs of each of the sub sections.
		for service := range deploy.cfg.GetStringMapString(scope) {
			errnie.Informs(fmt.Sprintf("deploying %s", service))
			// Apply the manifest to the cluster, using the values we
			// obtain from the key/value pairs.
			deploy.cluster.client.Apply(scope,
				deploy.cfg.GetStringMapString(
					fmt.Sprintf("%s.%s", scope, service),
				),
			)
		}
	}

	return nil
}

/*
DeployCmd deploys the service to a cluster.

This command is added to the rootCmd of each service, so we prevent
having to deplicate this code.

TODO: We could probably do the same with the runCmd, if we made all

	the managers have the same name, or better still, just have
	one manager that can handle all services, like the request handler.
*/
var DeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the service to a cluster.",
	Long:  deploytxt,
	RunE: func(_ *cobra.Command, _ []string) error {
		errnie.Tracing(tweaker.GetBool("errnie.trace"))
		errnie.Debugging(tweaker.GetBool("errnie.debug"))

		tags := []string{}

		// The tags of the container image. By default it should be
		// the stage (environment) that is configured in the configuration
		// file embedded in each service. For development we will use the
		// local user name.
		switch tweaker.Stage() {
		case "development":
			tags = append(tags, os.Getenv("USER"))
		default:
			tags = append(tags, tweaker.Stage())
		}

		// Build the container image for this service.
		container := NewContainer(tweaker.Program(), tags)
		container.Build()

		// Push the container image for this service to the container
		// registry the current user is logged in to.
		container.Push()

		// Builds a new KinD cluster if needed. If the cluster already
		// exists it will report an error, which can be safely ignored.
		cluster := NewKind()
		cluster.Provision()

		// Deploys the base infrastructure (if needed), and the services.
		// If any of these are already present, an upgrade is applied.
		// This will have a no-op effect if nothing changed.
		deploy := NewDeploy(cluster)
		deploy.Do()

		return nil
	},
}

/*
deploytxt lives here to keep the command definition section cleaner.
*/
var deploytxt = `
Builds the container for the correct stage.
Builds a cluster on the local machine if not present, or connects to
a cluster context from the kube config.
Deploys the base infrastructure and the service.
`
