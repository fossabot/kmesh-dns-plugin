/*
 * Copyright The Kmesh Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"os"

	"github.com/spf13/cobra"
	"istio.io/istio/pkg/cmd"
	logger "istio.io/istio/pkg/log"
	"kmesh.net/kmesh-coredns-plugin/pkg"
	"kmesh.net/kmesh-coredns-plugin/pkg/options"
)

var log = logger.RegisterScope("kmesh-dns", "kmesh-dns main")

func main() {
	cmd := newCommand()
	if err := cmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func newCommand() *cobra.Command {
	serveCmd := &cobra.Command{
		Use:          "kmesh-dns",
		Short:        "Start kmesh dns",
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			cmd.PrintFlags(c.Flags())
			m, err := pkg.NewDNSManager()
			if err != nil {
				return err
			}
			stop := make(chan struct{})
			if err := m.Start(stop); err != nil {
				return err
			}

			waitForMonitorSignal(stop)

			return nil
		},
	}

	options.GetConfig().AttachFlags(serveCmd)

	return serveCmd
}

func waitForMonitorSignal(stop chan struct{}) {
	cmd.WaitSignal(stop)
}
