package docker

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/golang/glog"
)

// Network is an structure for user-defined networks
type Network struct {
	Name   string `yaml:"name"`
	Subnet string `yaml:"subnet"`
}

// VerifyNetwork verifies a docker managed network
func VerifyNetwork(ctx context.Context, cli *client.Client, name string) error {
	filters := filters.NewArgs()
	filters.Add("name", name)
	_, err := cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters,
	})
	if err != nil {
		glog.Error(err)
		return err
	}
	return nil
}
