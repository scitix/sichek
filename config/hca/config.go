package hca

import (
	"github.com/scitix/sichek/components/infiniband/collector"
)

type HCASpec struct {
	HCAs map[string]*collector.IBHardWareInfo `json:"hca_spec"`
}
