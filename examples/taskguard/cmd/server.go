package cmd

import (
	"context"
	"flag"

	"github.com/scitix/taskguard/pkg/cfg"
	"github.com/scitix/taskguard/pkg/svc"
	"github.com/scitix/taskguard/pkg/taskguard"

	"github.com/zeromicro/go-zero/core/conf"
)

var configFile = flag.String("f", "etc/config.yaml", "the config file")

func Main() {
	flag.Parse()

	var c cfg.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	svcCtx := svc.NewServiceContext(c)
	ctx := context.Background()

	// create and start task guard for fault tolerance
	taskGuard := taskguard.MustNewController(svcCtx)
	taskGuard.RunOrDie(ctx)
}
