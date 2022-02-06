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

package connections

import (
	"bufio"
	"fmt"
	"github.com/kiran-sama/ebpf-training/workshop1/internal/structs"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Factory is a routine-safe container that holds a trackers with unique ID, and able to create new tracker.
type Factory struct {
	connections         map[structs.ConnID]*Tracker
	apiInventory        map[string]*ApiSchema
	inactivityThreshold time.Duration
	mutex               *sync.RWMutex
}

// NewFactory creates a new instance of the factory.
func NewFactory(inactivityThreshold time.Duration) *Factory {
	return &Factory{
		connections:         make(map[structs.ConnID]*Tracker),
		apiInventory:        make(map[string]*ApiSchema),
		mutex:               &sync.RWMutex{},
		inactivityThreshold: inactivityThreshold,
	}
}

func (factory *Factory) HandleReadyConnections() {
	trackersToDelete := make(map[structs.ConnID]struct{})

	for connID, tracker := range factory.connections {
		if tracker.IsComplete() {
			trackersToDelete[connID] = struct{}{}
			if len(tracker.sentBuf) == 0 && len(tracker.recvBuf) == 0 {
				continue
			}
			fmt.Printf("========================>\nFound HTTP payload\nRequest->\n%s\n\nResponse->\n%s\n\n<========================\n", tracker.recvBuf, tracker.sentBuf)
			reader := bufio.NewReader(strings.NewReader(string(tracker.sentBuf)))
			req, err := http.ReadRequest(reader)
			if err != nil {
				fmt.Printf(req.Method)
			}

		} else if tracker.Malformed() {
			trackersToDelete[connID] = struct{}{}
		} else if tracker.IsInactive(factory.inactivityThreshold) {
			trackersToDelete[connID] = struct{}{}
		}
	}
	factory.mutex.Lock()
	defer factory.mutex.Unlock()
	for key := range trackersToDelete {
		delete(factory.connections, key)
	}
}

func ParseRequest(requestJson string) (string, string, string) {

	return "", "", ""
}

func ParseResponse(responseJson string) (bool, string) {
	return false, ""
}

// GetOrCreate returns a tracker that related to the given connection and transaction ids. If there is no such tracker
// we create a new one.
func (factory *Factory) GetOrCreate(connectionID structs.ConnID) *Tracker {
	factory.mutex.Lock()
	defer factory.mutex.Unlock()
	tracker, ok := factory.connections[connectionID]
	if !ok {
		factory.connections[connectionID] = NewTracker(connectionID)
		return factory.connections[connectionID]
	}
	return tracker
}
