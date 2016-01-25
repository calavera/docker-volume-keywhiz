package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

        "github.com/docker/go-plugins-helpers/volume"
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
	m       *sync.Mutex
}

func newKeywhizDriver(root string, config keywhizConfig) keywhizDriver {
	return keywhizDriver{
		root:    root,
		config:  config,
		servers: map[string]*keywhizServer{},
		m:       &sync.Mutex{},
	}
}

func (d keywhizDriver) Create(r volume.Request) volume.Response {
	return volume.Response{}
}

func (d keywhizDriver) Remove(r volume.Request) volume.Response {
	d.m.Lock()
	defer d.m.Unlock()
	m := d.mountpoint(r.Name)

	if s, ok := d.servers[m]; ok {
		if s.connections <= 1 {
			delete(d.servers, m)
		}
	}
	return volume.Response{}
}

func (d keywhizDriver) Path(r volume.Request) volume.Response {
	return volume.Response{Mountpoint: d.mountpoint(r.Name)}
}

func (d keywhizDriver) Mount(r volume.Request) volume.Response {
	d.m.Lock()
	defer d.m.Unlock()
	m := d.mountpoint(r.Name)
	log.Printf("Mounting volume %s on %s\n", r.Name, m)

	s, ok := d.servers[m]
	if ok && s.connections > 0 {
		s.connections++
		return volume.Response{Mountpoint: m}
	}

	fi, err := os.Lstat(m)

	if os.IsNotExist(err) {
		if err := os.MkdirAll(m, 0755); err != nil {
			return volume.Response{Err: err.Error()}
		}
	} else if err != nil {
		return volume.Response{Err: err.Error()}
	}

	if fi != nil && !fi.IsDir() {
		return volume.Response{Err: fmt.Sprintf("%v already exist and it's not a directory", m)}
	}

	server, err := d.mountServer(m)
	if err != nil {
		return volume.Response{Err: err.Error()}
	}

	d.servers[m] = &keywhizServer{Server: server, connections: 1}

	return volume.Response{Mountpoint: m}
}

func (d keywhizDriver) Unmount(r volume.Request) volume.Response {
	d.m.Lock()
	defer d.m.Unlock()
	m := d.mountpoint(r.Name)
	log.Printf("Unmounting volume %s from %s\n", r.Name, m)

	if s, ok := d.servers[m]; ok {
		if s.connections == 1 {
			s.Unmount()
		}
		s.connections--
	} else {
		return volume.Response{Err: fmt.Sprintf("Unable to find volume mounted on %s", m)}
	}

	return volume.Response{}
}

func (d *keywhizDriver) mountpoint(name string) string {
	return filepath.Join(d.root, name)
}

func (d *keywhizDriver) mountServer(mountpoint string) (*fuse.Server, error) {
	logConfig := klog.Config{
		Debug:      d.config.Debug,
		Mountpoint: mountpoint,
	}

	if err := os.MkdirAll(filepath.Dir(mountpoint), 0755); err != nil {
		return nil, err
	}

	freshThreshold := 200 * time.Millisecond
	backendDeadline := 500 * time.Millisecond
	maxWait := d.config.TimeoutSeconds + backendDeadline
	timeouts := keywhizfs.Timeouts{
		Fresh:           freshThreshold,
		BackendDeadline: backendDeadline,
		MaxWait:         maxWait,
	}

	client := keywhizfs.NewClient(d.config.CertFile, d.config.KeyFile, d.config.CaFile,
		d.config.ServerURL, d.config.TimeoutSeconds, logConfig, d.config.Ping)

	ownership := keywhizfs.NewOwnership(d.config.User, d.config.Group)
	kwfs, root, err := keywhizfs.NewKeywhizFs(&client, ownership, timeouts, logConfig)
	if err != nil {
		client.Errorf("Mount fail: %v\n", err)
		return nil, err
	}

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
		return nil, err
	}

	go server.Serve()

	return server, nil
}
