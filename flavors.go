package main

import (
	"fmt"
	"os/exec"
)

type VMSize struct {
	CpuModel string `yaml:cpu-model`
	Sockets  int    `yaml:sockets`
	Cpus     int    `yaml:cpus`
	Cores    int    `yaml:cores`
	Threads  int    `yaml:threads`
	Ram      int    `yaml:ram`
}

func NewVMSize(model string, sockets, cpus, cores, threads, ram int) VMSize {
	var vmSize VMSize

	if model != "" {
		vmSize.CpuModel = model
	} else {
		if vmxSupport() {
			vmSize.CpuModel = "host"
		}
	}

	if sockets != 0 {
		vmSize.Sockets = sockets
	} else {
		vmSize.Sockets = 1
	}

	if cpus != 0 {
		vmSize.Cpus = cpus
	} else {
		vmSize.Cpus = 1
	}

	if cores != 0 {
		vmSize.Cores = cores
	} else {
		vmSize.Cores = 2
	}

	if threads != 0 {
		vmSize.Threads = threads
	} else {
		vmSize.Threads = 2
	}

	if ram != 0 {
		vmSize.Ram = ram
	} else {
		vmSize.Ram = 4096
	}

	return vmSize
}

func GetVMSizeFromFlavor(flavor string) VMSize {
	var size VMSize
	var cpuModel string

	if vmxSupport() {
		cpuModel = "host"
	} else {
		cpuModel = "haswell"
	}

	switch string(flavor) {
	case "micro":
		size = NewVMSize(cpuModel, 1, 1, 1, 1, 512)
	case "tiny":
		size = NewVMSize(cpuModel, 1, 1, 1, 1, 1024)
	case "small":
		size = NewVMSize(cpuModel, 1, 1, 2, 1, 2048)
	case "medium":
		size = NewVMSize(cpuModel, 1, 1, 2, 2, 4096)
	case "large":
		size = NewVMSize(cpuModel, 1, 1, 2, 2, 8192)
	default:
		size = NewVMSize(cpuModel, 1, 1, 2, 2, 4096)
	}
	return size
}

func vmxSupport() bool {
	err := exec.Command("grep", "-qw", "vmx", "/proc/cpuinfo").Run()
	if err != nil {
		fmt.Println(err)
		return false
	}
	return true
}
