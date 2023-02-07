// Copyright 2021 OnMetal authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/zapr"
	onmetalimage "github.com/onmetal/onmetal-image"
	cmdonmetalimage "github.com/onmetal/onmetal-image/cmd/onmetal-image"

	"go.uber.org/zap"

	"github.com/go-logr/logr"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ctx = onmetalimage.SetupContext(ctx)

	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	log := zapr.NewLogger(zapLog)

	ctx = logr.NewContext(ctx, log)
	if err := cmdonmetalimage.Command().ExecuteContext(ctx); err != nil {
		log.Error(err, "Error running command")
		os.Exit(1)
	}
}
