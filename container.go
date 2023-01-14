package infra

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/moby/moby/pkg/archive"
	"github.com/theapemachine/wrkspc/brazil"
	"github.com/theapemachine/wrkspc/tweaker"
	"github.com/wrk-grp/errnie"
)

type LogLine struct {
	Stream         string `json:"stream"`
	Status         string `json:"status"`
	ProgressDetail struct {
		Current  int    `json:"current"`
		Total    int    `json:"total"`
		Progress string `json:"progress"`
	} `json:"progressDetail"`
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

/*
Container wraps the behavior of building Docker images and pushing
them to container repositories.
*/
type Container struct {
	ctx  context.Context
	cli  *client.Client
	name string
	tags []string
}

/*
NewContainer constructs an instance of Container and returns a
pointer reference to it.

It is the entrypoint to building a Dockerfile and pushing the
result to a container registry.
*/
func NewContainer(name string, tags []string) *Container {
	// Setup a new client to the Docker daemon on the local machine.
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	errnie.Handles(err)

	return &Container{ctx, cli, name, tags}
}

/*
Build a Dockerfile for any service.
*/
func (container *Container) Build() {
	// Compress the Dockerfile context into a tarball.
	tar, err := archive.TarWithOptions(
		brazil.NewPath(".").Location, &archive.TarOptions{},
	)

	if errnie.Handles(err) != nil {
		return
	}

	// Login to the private Docker Hub registry.
	auth := types.AuthConfig{
		Username: tweaker.GetString("docker.username"),
		Password: tweaker.GetString("docker.password"),
	}

	_, err = container.cli.RegistryLogin(container.ctx, auth)
	if errnie.Handles(err) != nil {
		return
	}

	// Set the correct options and build the Docker container image.
	resp, err := container.cli.ImageBuild(
		container.ctx, tar, types.ImageBuildOptions{
			Dockerfile: "Dockerfile",
			Tags:       []string{"theapemachine/" + container.name + ":" + tweaker.Stage()},
			Remove:     true,
			PullParent: true,
			Platform:   "linux/amd64",
			AuthConfigs: map[string]types.AuthConfig{
				"https://index.docker.io/v1/": auth,
			},
		},
	)

	if errnie.Handles(err) != nil {
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	logLine := LogLine{}

	for scanner.Scan() {
		json.Unmarshal(scanner.Bytes(), &logLine)

		if logLine.Stream != "" {
			errnie.Informs(logLine.Stream)
		}

		if logLine.Status != "" {
			errnie.Debugs(logLine.Status)
		}

		if logLine.Error != "" {
			errnie.Handles(errors.New(logLine.Error))
			errnie.Handles(errors.New(logLine.ErrorDetail.Message))
		}
	}
}

func (container *Container) Push() {
	container.cli.ImagePush(
		container.ctx,
		fmt.Sprintf(
			"theapemachine/%s:%s",
			container.name, strings.Join(container.tags, ""),
		),
		types.ImagePushOptions{
			Platform: "linux/amd64",
		},
	)
}
