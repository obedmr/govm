package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/moby/moby/pkg/namesgenerator"
)

var vncPort string

type GoVM struct {
	Name        string   `yaml:"name"`
	ParentImage string   `yaml:"image"`
	Size        HostOpts `yaml:"size"`
	Cloud       bool     `yaml:"cloud"`
	Efi         bool     `yaml:"efi"`
	Workdir     string   `yaml:"workdir"`
	SSHKey      string   `yaml:"sshkey"`
	UserData    string   `yaml:"user-data"`

	containerID      string
	generateUserData bool
}

func NewGoVM(name, parentImage string, size HostOpts, cloud, efi bool, workdir string, publicKey string, userData string) GoVM {
	var govm GoVM
	var err error

	if parentImage == "" {
		fmt.Println("Missing --image argument")
		os.Exit(1)
	}
	govm.ParentImage, err = filepath.Abs(parentImage)
	if err != nil {
		fmt.Printf("Unable to determine image location: %v\n", err)
		os.Exit(1)
	}
	err = saneImage(parentImage)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	/* Optional Flags */
	if name != "" {
		govm.Name = name
	} else {
		govm.Name = namesgenerator.GetRandomName(0)
	}

	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	_, err = cli.ContainerInspect(ctx, name)
	if err == nil {
		log.Fatal("There is an existing container with the same name")
	}
	// Check the workdir
	if workdir != "" {
		govm.Workdir = workdir
	} else {
		govm.Workdir = wdir
	}

	// Check if user data is provided
	if userData != "" {
		absUserData, err := filepath.Abs(userData)
		if err != nil {
			fmt.Printf("Unable to determine %s user data file location: %v\n", govm, err)
			os.Exit(1)
		}
		// Test if the template file exists
		_, err = os.Stat(absUserData)
		if err != nil {
			// Look for a script verifying the shebang
			var validShebang bool
			validShebangs := []string{
				"#cloud-config",
				"#!/bin/sh",
				"#!/bin/bash",
				"#!/usr/bin/env python",
			}
			_, shebang, _ := bufio.ScanLines([]byte(userData), true)
			for _, sb := range validShebangs {
				if string(shebang) == sb {
					validShebang = true
				}
			}
			if validShebang == true {
				govm.generateUserData = true
				govm.UserData = userData
			} else {
				fmt.Println("Unable to determine the user data content")
				os.Exit(1)
			}

		} else {
			govm.UserData = absUserData
		}
	}

	// Check if any flavor is provided
	if size != "" {
		govm.Size = getFlavor(string(size))
	} else {
		govm.Size = getFlavor("")
	}

	// Check if efi flag is provided
	if efi != false {
		govm.Efi = efi
	}

	// Check if cloud flag is provided
	if cloud != false {
		govm.Cloud = cloud

	}

	if publicKey != "" {
		key, err := ioutil.ReadFile(publicKey)
		if err != nil {
			log.Fatal(err)
		}
		govm.SSHKey = string(key)
	} else {
		key, err := ioutil.ReadFile(keyPath)
		if err != nil {
			log.Fatal(err)
		}
		govm.SSHKey = string(key)
	}

	return govm
}

func (govm *GoVM) ShowInfo() {
	ctx := context.Background()

	// Create the Docker API client
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	containerInfo, _ := cli.ContainerInspect(ctx, govm.containerID)
	fmt.Printf("[%s]\nIP Address: %s\n", containerInfo.Name[1:], containerInfo.NetworkSettings.DefaultNetworkSettings.IPAddress)

}

func (govm *GoVM) setVNC(govmName string, port string) {
}

func (govm *GoVM) Launch() {
	ctx := context.Background()

	// Create the data dir
	vmDataDirectory := govm.Workdir + "/data/" + govm.Name
	err := os.MkdirAll(vmDataDirectory, 0740)
	if err != nil {
		fmt.Printf("Unable to create: %s", vmDataDirectory)
		os.Exit(1)
	}

	// Create the metadata file
	vmMetaData := ConfigDriveMetaData{
		"govm",
		govm.Name,
		"0",
		govm.Name,
		map[string]string{},
		map[string]string{
			"mykey": govm.SSHKey,
		},
		"0",
	}

	vmMetaDataJSON, err := json.Marshal(vmMetaData)
	if err != nil {
		log.Fatal(err)
	}

	err = ioutil.WriteFile(vmDataDirectory+"/meta_data.json", vmMetaDataJSON, 0664)
	if err != nil {
		log.Fatal(err)
	}

	// Create the user_data file
	if govm.generateUserData == true {
		err = ioutil.WriteFile(vmDataDirectory+"/user_data", []byte(govm.UserData), 0664)
		if err != nil {
			log.Fatal(err)
		}
		govm.UserData = vmDataDirectory + "/user_data"
	}

	// Default Enviroment Variables
	env := []string{
		"AUTO_ATTACH=yes",
		"DEBUG=yes",
		fmt.Sprintf("KVM_CPU_OPTS=%v", govm.Size),
	}
	if host_dns {
		env = append(env, "ENABLE_DHCP=no")
	}

	/* QEMU ARGUMENTS PASSED TO THE CONTAINER */
	qemuParams := []string{
		"-vnc unix:/data/vnc",
	}
	if govm.Efi {
		qemuParams = append(qemuParams, "-bios /OVMF.fd ")
	}
	if govm.Cloud {
		env = append(env, "CLOUD=yes")
		env = append(env, "CLOUD_INIT_OPTS=-drive file=/data/seed.iso,if=virtio,format=raw ")
	}

	// Default Mount binds
	defaultMountBinds := []string{
		fmt.Sprintf("%v:/image/image", govm.ParentImage),
		fmt.Sprintf("%v:/data", vmDataDirectory),
		fmt.Sprintf("%v:/cloud-init/openstack/latest/meta_data.json", vmDataDirectory+"/meta_data.json"),
	}

	if govm.UserData != "" {
		defaultMountBinds = append(defaultMountBinds, fmt.Sprintf("%s:/cloud-init/openstack/latest/user_data", govm.UserData))
	}

	// Create the Docker API client
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	// Get the govm/govm image

	//_, err = cli.ImagePull(ctx, "govm", types.ImagePullOptions{})
	//if err != nil {
	//	panic(err)
	//}

	/* WIP Exposed Ports
	// Default Ports
	var ports nat.PortMap
	var exposedPorts nat.PortSet
	vncPort := "5910"
	_, ports, _ = nat.ParsePortSpecs([]string{
		fmt.Sprintf(":%v:%v", vncPort, vncPort),
	})

	exposedPorts = map[nat.Port]struct{}{
	      "5910/tcp": {},
	}
	*/

	// Get an available port for VNC
	vncPort = strconv.Itoa(findPort())

	// Create the Container
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: "verbacious/govm",
		Cmd:   qemuParams,
		Env:   env,
		Labels: map[string]string{
			"websockifyPort": vncPort,
			"dataDir":        vmDataDirectory,
		},
	}, &container.HostConfig{
		Privileged:      true,
		PublishAllPorts: true,
		Binds:           defaultMountBinds,
	}, nil, govm.Name)
	if err != nil {
		panic(err)
	}

	govm.containerID = resp.ID

	// Start the container
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	govm.setVNC(govm.Name, vncPort)
}
