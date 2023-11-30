// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	ironcoreimage "github.com/ironcore-dev/ironcore-image"
	cmdironcoreimage "github.com/ironcore-dev/ironcore-image/cmd/ironcore-image"
	"go.uber.org/zap"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ctx = ironcoreimage.SetupContext(ctx)

	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	log := zapr.NewLogger(zapLog)

	ctx = logr.NewContext(ctx, log)
	if err := cmdironcoreimage.Command().ExecuteContext(ctx); err != nil {
		log.Error(err, "Error running command")
		os.Exit(1)
	}
}
