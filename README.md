# Docker Keywhiz-fs volume extension

Mount Keywhiz-fs inside your contaniners talking to a remote Keywhiz server.

The FUSE mount point is shared between containers if the name of the volume is the same between containers.
Otherwhise, a new volume is mounted per container.

## Installation

Using go (until we have proper binaries):

```
$ go get github.com/calavera/docker-volume-keywhiz
```

## Usage

1. Run the daemon and connect to a Keywhiz server:

```
$ sudo docker-volume-keywhiz keywhiz_server_url
```

2. Run containers pointing to the driver:

```
$ docker run --volume-driver keywhiz-fs --volumev all-my-secrets:/etc/secrets -it alpine ls /etc/secrets/
```

## LICENSE

MIT
