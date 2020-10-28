package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"syscall"

	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	AESMSocketDir = "/var/run/aesmd"
)

var allMounts = map[string]bool{
	// optional
	AESMSocketDir: false,
}

var allDevices = map[string]bool{
	// required
	"/dev/isgx": false,
	// optional
	"/dev/gsgx": false,
}

type arguments struct {
	bundleDir string
	cmd       string
}

var debugLog = "/dev/null"

var fileLogger *log.Logger = nil

// parseArguments get arguments from docker
func parseArguments() (*arguments, error) {
	args := &arguments{}

	for i, param := range os.Args {
		switch param {
		case "--bundle", "-b":
			if len(os.Args)-i <= 1 {
				return nil, fmt.Errorf("bundle option needs an arguments")
			}
			args.bundleDir = os.Args[i+1]
		case "create":
			args.cmd = param
		}
	}

	return args, nil
}

// exit print error message and exit
func exit(msg string, err error) {
	if fileLogger != nil {
		fileLogger.Printf("%s: %s, %v", os.Args[0], msg, err)
		return
	}
	log.Fatalf("%s: %s, %v\n", os.Args[0], msg, err)
}

func execRunc() {
	fileLogger.Println("Looking for \"docker-runc\" binary")
	runcPath, err := exec.LookPath("docker-runc")
	if err != nil {
		fileLogger.Println("\"docker-runc\" binary not found")
		fileLogger.Println("Looking for \"runc\" binary")
		runcPath, err = exec.LookPath("runc")
		if err != nil {
			exit("find runc path", err)
		}
	}

	fileLogger.Printf("Runc path: %s\n", runcPath)

	err = syscall.Exec(runcPath, append([]string{runcPath}, os.Args[1:]...), os.Environ())
	if err != nil {
		exit("exec runc binary", err)
	}
}

// addSGXSpecification add necessary devices and mountpoints for SGX
func addSGXSpecification(spec *specs.Spec) {
	// Detecting mount points
	for mnt := range allMounts {
		if fi, err := os.Stat(mnt); err == nil && fi.IsDir() {
			allMounts[mnt] = true
		}
	}

	// Detecting device
	for dev := range allDevices {
		if fi, err := os.Stat(dev); err == nil && !fi.IsDir() {
			allDevices[dev] = true
		}
	}

	mounted := make(map[string]bool)
	for _, mnt := range spec.Mounts {
		mounted[mnt.Source] = true
	}

	for mnt, enabled := range allMounts {
		_, existed := mounted[mnt]
		if !existed && enabled {
			spec.Mounts = append(spec.Mounts, specs.Mount{
				Source:      mnt,
				Destination: mnt,
				Type:        "bind",
				Options:     []string{"rbind", "rprivate"},
			})
		}
	}

	if spec.Linux != nil {
		devs := make(map[string]bool)
		for _, dev := range spec.Linux.Devices {
			devs[dev.Path] = true
		}

		for dev, enabled := range allDevices {
			_, existed := devs[dev]
			if !existed && enabled {
				di, _ := devices.DeviceFromPath(dev, "rmw")
				spec.Linux.Devices = append(spec.Linux.Devices, specs.LinuxDevice{
					Path:     di.Path,
					Type:     string(di.Type),
					Major:    di.Major,
					Minor:    di.Minor,
					FileMode: &di.FileMode,
					UID:      &di.Uid,
					GID:      &di.Uid,
				})

				spec.Linux.Resources.Devices = append(spec.Linux.Resources.Devices, specs.LinuxDeviceCgroup{
					Allow:  true,
					Type:   string(di.Type),
					Major:  &di.Major,
					Minor:  &di.Minor,
					Access: "rmw",
				})
			}
		}
	}
}

func main() {
	logFile, err := os.OpenFile(debugLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		exit(fmt.Sprintf("open %s", debugLog), err)
		return
	}
	defer logFile.Close()

	fileLogger = log.New(logFile, "", log.LstdFlags)
	fileLogger.Printf("Running %v\n", os.Args)

	args, err := parseArguments()
	if err != nil {
		exit("can't parse arguments", err)
		return
	}

	if args.cmd == "create" {
		if args.bundleDir == "" {
			args.bundleDir, err = os.Getwd()
			if err != nil {
				exit("get working directory", err)
				return
			}
			fileLogger.Printf("Bundle directory path is empty, using working directory: %s\n", args.bundleDir)
		}

		configFile, err := os.OpenFile(path.Join(args.bundleDir, "config.json"), os.O_RDWR, 0644)
		if err != nil {
			exit("open OCI spec file", err)
			return
		}
		defer configFile.Close()

		specData, err := ioutil.ReadAll(configFile)
		if err != nil {
			exit("read spec data", err)
			return
		}

		var spec specs.Spec
		err = json.Unmarshal(specData, &spec)
		if err != nil {
			exit("decode OCI spec file", err)
			return
		}

		addSGXSpecification(&spec)
		newSpecData, err := json.Marshal(&spec)
		if err != nil {
			exit("encode OCI spec file", err)
			return
		}

		_, err = configFile.WriteAt(newSpecData, 0)
		if err != nil {
			exit("write OCI spec file", err)
			return
		}
	}

	execRunc()
}
