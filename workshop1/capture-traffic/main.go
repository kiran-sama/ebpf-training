/*
 * Copyright 2018- The Pixie Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"fmt"
	bpfwrapper2 "github.com/kiran-sama/ebpf-training/workshop1/internal/bpfwrapper"
	"github.com/kiran-sama/ebpf-training/workshop1/internal/connections"
	"github.com/kiran-sama/ebpf-training/workshop1/internal/settings"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"os/user"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/iovisor/gobpf/bcc"
)

// abortIfNotRoot checks the current user permissions, if the permissions are not elevated, we abort.
func abortIfNotRoot() {
	current, err := user.Current()
	if err != nil {
		log.Panic(err)
	}

	if current.Uid != "0" {
		log.Panic("sniffer must run under superuser privileges")
	}
}

// recoverFromCrashes is a defer function that caches all panics being thrown from the application.
func recoverFromCrashes() {
	if err := recover(); err != nil {
		log.Printf("Application crashed: %v\nstack: %s\n", err, string(debug.Stack()))
	}
}

// updateAllowedPIDs updates the BPF map by adding the given PIDs
func updateAllowedPIDs(table *bcc.Table, pids []uint32) error {
	for _, pid := range pids {
		key := make([]byte, 4)
		value := make([]byte, 1)

		// Convert pid to a byte array to use as a key
		bcc.GetHostByteOrder().PutUint32(key, pid)

		// The value is arbitrary, for example purposes we set it to 1
		value[0] = 1

		err := table.Set(key, value)
		if err != nil {
			return fmt.Errorf("failed to update allowed_pids map for PID %d: %v", pid, err)
		}
		fmt.Printf("Added PID %d to allowed_pids map\n", pid)
	}
	return nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run main.go <path to bpf source code>")
		os.Exit(1)
	}
	bpfSourceCodeFile := os.Args[1]
	bpfSourceCodeContent, err := ioutil.ReadFile(bpfSourceCodeFile)
	if err != nil {
		log.Panic(err)
	}

	defer recoverFromCrashes()
	abortIfNotRoot()

	if err := settings.InitRealTimeOffset(); err != nil {
		log.Printf("Failed fixing BPF clock, timings will be offseted: %v", err)
	}


	// Catching all termination signals to perform a cleanup when being stopped.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	bpfModule := bcc.NewModule(string(bpfSourceCodeContent), nil)
	if bpfModule == nil {
		log.Panic("bpf is nil")
	}
	defer bpfModule.Close()


	// Load the BPF map (allowed_pids)
	mapFD := bpfModule.TableId("allowed_pids")
	allowedPidsTable := bcc.NewTable(mapFD, bpfModule)

	// Example PIDs to update
	allowedPIDs := []uint32{6743, 8338}

	// Update PIDs in the BPF map
	err = updateAllowedPIDs(allowedPidsTable, allowedPIDs)
	if err != nil {
		log.Fatalf("Failed to update allowed PIDs: %v", err)
	}

	connectionFactory := connections.NewFactory(time.Minute)
	go func() {
		for {
			connectionFactory.HandleReadyConnections()
			time.Sleep(10 * time.Second)
		}
	}()
	if err := bpfwrapper2.LaunchPerfBufferConsumers(bpfModule, connectionFactory); err != nil {
		log.Panic(err)
	}

	// Lastly, after everything is ready and configured, attach the kprobes and start capturing traffic.
	if err := bpfwrapper2.AttachKprobes(bpfModule); err != nil {
		log.Panic(err)
	}
	log.Println("Sniffer is ready")
	<-sig
	log.Println("Signaled to terminate")
}
