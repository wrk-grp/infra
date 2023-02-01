package infra

import (
	"context"
	"fmt"
	"log"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
)

func main() {
	// Connect to containerd
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Use a context with a timeout
	ctx := namespaces.WithNamespace(context.Background(), "example")
	timeoutCtx, cancel := context.WithTimeout(ctx, 60*1000*1000*1000)
	defer cancel()

	// Pull an image
	image, err := client.Pull(timeoutCtx, "docker.io/library/alpine:latest", containerd.WithPullUnpack)
	if err != nil {
		log.Fatal(err)
	}

	// Create a new container
	container, err := client.NewContainer(timeoutCtx, "example",
		containerd.WithNewSnapshot("example-snapshot", image),
		containerd.WithNewSpec(oci.WithImageConfig(image)),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer container.Delete(timeoutCtx, containerd.WithSnapshotCleanup)

	// Start the container
	task, err := container.NewTask(timeoutCtx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		log.Fatal(err)
	}
	defer task.Delete(timeoutCtx)

	// Start the task
	if err := task.Start(timeoutCtx); err != nil {
		log.Fatal(err)
	}

	// Wait for the task to exit
	status, err := task.Wait(timeoutCtx)
	if err != nil {
		log.Fatal(err)
	}

	// Print the exit status
	fmt.Println("Exit status:", status)
}
