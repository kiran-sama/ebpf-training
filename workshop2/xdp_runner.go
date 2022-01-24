// Drop incoming packets on XDP layer and count for which
// protocol type. Based on:
// https://github.com/iovisor/bcc/blob/master/examples/networking/xdp/xdp_drop_count.py
//
// Copyright (c) 2017 GustavoKatel
// Licensed under the Apache License, Version 2.0 (the "License")

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"

	"github.com/iovisor/gobpf/bcc"
)

/*
#cgo CFLAGS: -I/usr/include/bcc/compat
#cgo LDFLAGS: -lbcc
#include <bcc/bcc_common.h>
#include <bcc/libbpf.h>
void perf_reader_free(void *ptr);
*/
import "C"

// protocols contains minimal mappings between internet protocol
// names and numbers for platforms that don't have a complete list of
// protocol numbers.
//
// See https://www.iana.org/assignments/protocol-numbers
//
// On Unix, this map is augmented by readProtocols via lookupProtocol.

var protocols = map[uint32]string{
	0:  "HOPOPT",
	1:  "icmp",
	2:  "igmp",
	6:  "tcp",
	17: "udp",
	58: "ipv6-icmp",
}

func usage() {
	fmt.Printf("Usage: sudo %v <xdp bpf code> <ifdev>\n", os.Args[0])
	fmt.Printf("e.g.: sudo %v xdp_prog.c lo\n", os.Args[0])
	os.Exit(1)
}

func main() {
	var device string

	if len(os.Args) != 3 {
		usage()
	}

	bpfSourceCodeFile := os.Args[1]
	bpfSourceCodeContent, err := ioutil.ReadFile(bpfSourceCodeFile)
	if err != nil {
		log.Panic(err)
	}

	device = os.Args[2]

	ret := "XDP_DROP"

	module := bcc.NewModule(string(bpfSourceCodeContent), []string{
		"-w",
		"-DRETURNCODE=" + ret,
	})
	defer module.Close()

	fn, err := module.Load("xdp_dropper", C.BPF_PROG_TYPE_XDP, 1, 65536)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load xdp prog: %v\n", err)
		os.Exit(1)
	}

	err = module.AttachXDP(device, fn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to attach xdp prog: %v\n", err)
		os.Exit(1)
	}

	defer func() {
		if err := module.RemoveXDP(device); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to remove XDP from %s: %v\n", device, err)
		}
	}()

	fmt.Println("Dropping packets, hit CTRL+C to stop")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	dropcnt := bcc.NewTable(module.TableId("dropcnt"), module)

	<-sig

	fmt.Printf("\n{IP protocol-number}: {total dropped pkts}\n")
	for it := dropcnt.Iter(); it.Next(); {
		key := protocols[bcc.GetHostByteOrder().Uint32(it.Key())]
		if key == "" {
			key = "Unknown"
		}
		value := bcc.GetHostByteOrder().Uint64(it.Leaf())

		if value > 0 {
			fmt.Printf("%v: %v pkts\n", key, value)
		}
	}
}
