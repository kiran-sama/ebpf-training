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
	"io"
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
	factory.mutex.Lock()
	defer factory.mutex.Unlock()
	for connID, tracker := range factory.connections {
		if tracker.IsComplete() {
			trackersToDelete[connID] = struct{}{}
			if len(tracker.sentBuf) == 0 && len(tracker.recvBuf) == 0 {
				continue
			}
			fmt.Printf("========================>\nFound HTTP payload\nRequest->\n%s\n\nResponse->\n%s\n\n<========================\n", tracker.recvBuf, tracker.sentBuf)
			reader := bufio.NewReader(strings.NewReader(string(tracker.recvBuf)))
			req, e1 := http.ReadRequest(reader)
			reader = bufio.NewReader(strings.NewReader(string(tracker.sentBuf)))
			res, e2 := http.ReadResponse(reader, req)
			if e1 == nil && e2 == nil {
				if res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Type"), "application/json") {
					continue
				} else {
					if _, ok := factory.apiInventory[req.Method+"_"+req.RequestURI]; !ok {
						fmt.Println("New URI found")
						fmt.Println(req.Method)
						fmt.Println(req.RequestURI)
						// Building Schema
						requestSchemaBytes, _ := io.ReadAll(req.Body)
						requestSchema := string(requestSchemaBytes)
						responseSchemaBytes, _ := io.ReadAll(res.Body)
						responseSchema := string(responseSchemaBytes)
						fmt.Println(requestSchema)
						fmt.Println(responseSchema)
						factory.apiInventory[req.Method+"_"+req.RequestURI] = NewApiSchema(
							req.Method, req.RequestURI, requestSchema, responseSchema, false)
					}
				}
			} else {
				fmt.Println("Error building request/response")
			}
		} else if tracker.Malformed() {
			trackersToDelete[connID] = struct{}{}
		} else if tracker.IsInactive(factory.inactivityThreshold) {
			trackersToDelete[connID] = struct{}{}
		}
	}
	for key := range trackersToDelete {
		delete(factory.connections, key)
	}
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
