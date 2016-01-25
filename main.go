package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

        "github.com/docker/go-plugins-helpers/volume"
	klog "github.com/square/keywhiz-fs/log"
	"golang.org/x/sys/unix"
)

const (
	keywhizId     = "_keywhiz"
	socketAddress = "/run/docker/plugins/keywhiz.sock"
)

var (
	defaultPath = filepath.Join(volume.DefaultDockerRootDirectory, keywhizId)

	root           = flag.String("root", defaultPath, "Docker volumes root directory")
	certFile       = flag.String("cert", "", "PEM-encoded certificate file")
	keyFile        = flag.String("key", "client.key", "PEM-encoded private key file")
	caFile         = flag.String("ca", "cacert.crt", "PEM-encoded CA certificates file")
	user           = flag.String("asuser", "keywhiz", "Default user to own files")
	group          = flag.String("group", "keywhiz", "Default group to own files")
	ping           = flag.Bool("ping", false, "Enable startup ping to server")
	debug          = flag.Bool("debug", false, "Enable debugging output")
	timeoutSeconds = flag.Uint("timeout", 20, "Timeout for communication with server")
)

func main() {
	var Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] url\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()
	if flag.NArg() != 1 {
		Usage()
		os.Exit(1)
	}

	config := keywhizConfig{
		ServerURL:      flag.Args()[0],
		CertFile:       *certFile,
		KeyFile:        *keyFile,
		CaFile:         *caFile,
		User:           *user,
		Group:          *group,
		Ping:           *ping,
		Debug:          *debug,
		TimeoutSeconds: time.Duration(*timeoutSeconds) * time.Second,
	}

	lockMemory(config.Debug)

	d := newKeywhizDriver(*root, config)
	h := volume.NewHandler(d)
	fmt.Printf("Listening on %s\n", socketAddress)
	fmt.Println(h.ServeUnix("root", socketAddress))
}

// Locks memory, preventing memory from being written to disk as swap
func lockMemory(debug bool) {
	logConfig := klog.Config{
		Debug:      debug,
		Mountpoint: "",
	}
	logger := klog.New("kwfs_main", logConfig)

	err := unix.Mlockall(unix.MCL_FUTURE | unix.MCL_CURRENT)
	switch err {
	case nil:
	case unix.ENOSYS:
		logger.Warnf("mlockall() not implemented on this system")
	case unix.ENOMEM:
		logger.Warnf("mlockall() failed with ENOMEM")
	default:
		log.Fatalf("Could not perform mlockall and prevent swapping memory: %v", err)
	}
}
