// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package e2e

import (
	"fmt"
	"golang.org/x/net/context"
	"os"
	"runtime/pprof"
	"testing"
	"time"
)

const TIMES_TO_RUN_EACH_TEST = 2

func TestMain(m *testing.M) {
	exitCode := 0

	config := GetConfig()
	if config.Bootstrap {
		tl := NewLoggerRandomer()

		appNetwork := NewInProcessE2EAppNetwork(config.AppVcid, tl, "")

		exitCode = m.Run()
		appNetwork.GracefulShutdownAndWipeDisk()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		defer func() { // TODO remove once https://github.com/orbs-network/govnr/issues/8 is resolved
			if err := recover(); err != nil {
				printErrorAndStackTraces(err)
			}
		}()

		appNetwork.WaitUntilShutdown(shutdownCtx)
	} else {
		exitCode = m.Run()
	}

	os.Exit(exitCode)
}

func printErrorAndStackTraces(err interface{}) {
	fmt.Printf("Error waiting for system shutdown: %v\n", err)
	fmt.Println("------------------------------------------")
	fmt.Println("Locking goroutines: ")
	fmt.Println("------------------------------------------")
	pprof.Lookup("block").WriteTo(os.Stdout, 1)
	fmt.Println()
	fmt.Println("------------------------------------------")
	fmt.Println("All goroutines: ")
	fmt.Println("------------------------------------------")
	pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
}

func runMultipleTimes(t *testing.T, f func(t *testing.T)) {
	for i := 0; i < TIMES_TO_RUN_EACH_TEST; i++ {
		name := fmt.Sprintf("%s_#%d", t.Name(), i+1)
		t.Run(name, f)
		time.Sleep(100 * time.Millisecond) // give async processes time to separate between iterations
	}
}
