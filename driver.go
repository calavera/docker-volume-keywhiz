package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/calavera/docker-volume-api"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/square/keywhiz-fs"
	klog "github.com/square/keywhiz-fs/log"
)

type keywhizConfig struct {
	ServerURL      string
	CertFile       string
	KeyFile        string
	CaFile         string
	User           string
	Group          string
	Ping           bool
	Debug          bool
	TimeoutSeconds time.Duration
}

type keywhizServer struct {
	*fuse.Server
	connections int
}

type keywhizDriver struct {
	root    string
	config  keywhizConfig
	servers map[string]*keywhizServer
	m       sync.Mutex
}

func newKeywhizDriver(root string, config keywhizConfig) keywhizDriver {
	return keywhizDriver{
		root:    root,
		config:  config,
		servers: map[string]*keywhizServer{}}
}

func (d keywhizDriver) Create(r volumeapi.VolumeRequest) volumeapi.VolumeResponse {
	return volumeapi.VolumeResponse{}
}

func (d keywhizDriver) Remove(r volumeapi.VolumeRequest) volumeapi.VolumeResponse {
	d.m.Lock()
	defer d.m.Unlock()
	m := d.mountpoint(r.Name)

	if s, ok := d.servers[m]; ok {
		if s.connections == 1 {
			delete(d.servers, m)
		}
	}
	return volumeapi.VolumeResponse{}
}

func (d keywhizDriver) Path(r volumeapi.VolumeRequest) volumeapi.VolumeResponse {
	return volumeapi.VolumeResponse{Mountpoint: d.mountpoint(r.Name)}
}

func (d keywhizDriver) Mount(r volumeapi.VolumeRequest) volumeapi.VolumeResponse {
	d.m.Lock()
	defer d.m.Unlock()
	m := d.mountpoint(r.Name)

	fi, err := os.Lstat(m)

	if err != nil && os.IsNotExist(err) {
		return d.mountServer(m)
	} else if err != nil {
		return volumeapi.VolumeResponse{Err: err}
	}

	if !fi.IsDir() {
		return volumeapi.VolumeResponse{Err: fmt.Errorf("%v already exist and it's not a directory", m)}
	}

	s, ok := d.servers[m]
	if !ok {
		return volumeapi.VolumeResponse{Err: fmt.Errorf("fuse server destroyed")}
	}
	s.connections++

	return volumeapi.VolumeResponse{Mountpoint: m}
}

func (d keywhizDriver) Umount(r volumeapi.VolumeRequest) volumeapi.VolumeResponse {
	d.m.Lock()
	defer d.m.Unlock()
	mountpoint := d.mountpoint(r.Name)

	if s, ok := d.servers[mountpoint]; ok {
		if s.connections == 1 {
			err := s.Unmount()
			return volumeapi.VolumeResponse{Err: err}
		} else {
			s.connections--
		}
	}

	return volumeapi.VolumeResponse{}
}

func (d *keywhizDriver) mountpoint(name string) string {
	return filepath.Join(d.root, name)
}

func (d *keywhizDriver) mountServer(mountpoint string) volumeapi.VolumeResponse {
	logConfig := klog.Config{d.config.Debug, mountpoint}

	freshThreshold := 200 * time.Millisecond
	backendDeadline := 500 * time.Millisecond
	maxWait := d.config.TimeoutSeconds + backendDeadline
	timeouts := keywhizfs.Timeouts{freshThreshold, backendDeadline, maxWait}

	client := keywhizfs.NewClient(d.config.CertFile, d.config.KeyFile, d.config.CaFile,
		d.config.ServerURL, d.config.TimeoutSeconds, logConfig, d.config.Ping)

	ownership := keywhizfs.NewOwnership(d.config.User, d.config.Group)
	kwfs, root, err := keywhizfs.NewKeywhizFs(&client, ownership, timeouts, logConfig)
	mountOptions := &fuse.MountOptions{
		AllowOther: true,
		Name:       kwfs.String(),
		Options:    []string{"default_permissions"},
	}

	// Empty Options struct avoids setting a global uid/gid override.
	conn := nodefs.NewFileSystemConnector(root, &nodefs.Options{})
	server, err := fuse.NewServer(conn.RawFS(), mountpoint, mountOptions)
	if err != nil {
		client.Errorf("Mount fail: %v\n", err)
		return volumeapi.VolumeResponse{Err: err}
	}

	d.servers[mountpoint] = &keywhizServer{server, 1}
	go server.Serve()

	return volumeapi.VolumeResponse{Mountpoint: mountpoint}
}
