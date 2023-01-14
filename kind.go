package infra

import (
	"github.com/theapemachine/wrkspc/brazil"
	"sigs.k8s.io/kind/cmd/kind/app"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/log"
)

type Kind struct {
	client *Client
}

func NewKind() *Kind {
	return &Kind{}
}

func (cluster *Kind) Provision() error {
	app.Run(
		log.NoopLogger{}, cmd.StandardIOStreams(), []string{
			"create",
			"cluster",
			"--name", "cc-kind",
			"--config", brazil.NewPath("~/.kind-config.yml").Location,
		},
	)

	cluster.client = NewClient()
	return nil
}

func (cluster *Kind) Teardown() error {
	return nil
}
