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
	structs2 "github.com/kiran-sama/ebpf-training/workshop1/internal/structs"
	"log"
	"sync"
	"time"
)

const (
	maxBufferSize = 100 * 1024 // 100KB
)

type Tracker struct {
	connID structs2.ConnID

	addr              structs2.SockAddrIn
	openTimestamp     uint64
	closeTimestamp    uint64
	totalWrittenBytes uint64
	totalReadBytes    uint64

	// Indicates the tracker stopped tracking due to closing the session.
	lastActivityTimestamp uint64
	sentBytes             uint64
	recvBytes             uint64

	recvBuf []byte
	sentBuf []byte
	mutex   sync.RWMutex
}

func NewTracker(connID structs2.ConnID) *Tracker {
	return &Tracker{
		connID:  connID,
		recvBuf: make([]byte, 0, maxBufferSize),
		sentBuf: make([]byte, 0, maxBufferSize),
		mutex:   sync.RWMutex{},
	}
}

func (conn *Tracker) ToBytes() ([]byte, []byte) {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()
	return conn.recvBuf, conn.sentBuf
}

func (conn *Tracker) IsInactive(duration time.Duration) bool {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()
	return uint64(time.Now().UnixNano())-conn.lastActivityTimestamp > uint64(duration.Nanoseconds())
}

func (conn *Tracker) IsComplete() bool {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()
	return conn.closeTimestamp != 0 &&
		conn.totalReadBytes == conn.recvBytes &&
		conn.totalWrittenBytes == conn.sentBytes
}

func (conn *Tracker) Malformed() bool {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()
	return conn.closeTimestamp != 0 &&
		conn.totalReadBytes != conn.recvBytes &&
		conn.totalWrittenBytes != conn.sentBytes
}

func (conn *Tracker) AddDataEvent(event structs2.SocketDataEvent) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	conn.updateTimestamps()

	switch event.Attr.Direction {
	case structs2.EgressTraffic:
		conn.sentBuf = append(conn.sentBuf, event.Msg[:event.Attr.MsgSize]...)
		conn.sentBytes += uint64(event.Attr.MsgSize)
	case structs2.IngressTraffic:
		conn.recvBuf = append(conn.recvBuf, event.Msg[:event.Attr.MsgSize]...)
		conn.recvBytes += uint64(event.Attr.MsgSize)
	default:
	}
}

func (conn *Tracker) AddOpenEvent(event structs2.SocketOpenEvent) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	conn.updateTimestamps()
	conn.addr = event.Addr
	if conn.openTimestamp != 0 && conn.openTimestamp != event.TimestampNano {
		log.Printf("Changed open info timestamp from %v to %v", conn.openTimestamp, event.TimestampNano)
	}
	conn.openTimestamp = event.TimestampNano
}

func (conn *Tracker) AddCloseEvent(event structs2.SocketCloseEvent) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	conn.updateTimestamps()
	if conn.closeTimestamp != 0 && conn.closeTimestamp != event.TimestampNano {
		log.Printf("changed close info timestamp from %v to %v", conn.closeTimestamp, event.TimestampNano)
	}
	conn.closeTimestamp = event.TimestampNano

	conn.totalWrittenBytes = uint64(event.WrittenBytes)
	conn.totalReadBytes = uint64(event.ReadBytes)
}

func (conn *Tracker) updateTimestamps() {
	conn.lastActivityTimestamp = uint64(time.Now().UnixNano())
}
