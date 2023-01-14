package infra

type Cluster interface {
	Provision() error
	Teardown() error
}
