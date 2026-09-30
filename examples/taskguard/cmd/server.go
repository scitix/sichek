/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
	taskguard.MustNewController(svcCtx).RunOrDie(ctx)
}
