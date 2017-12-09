//+build !windows

package hostports

import "github.com/rancher/go-rancher-metadata/metadata"

func Watch(c metadata.Client) error { return nil }
