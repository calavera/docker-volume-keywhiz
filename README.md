# Docker Keywhiz-fs volume extension

Mount Keywhiz-fs inside your contaniners talking to a remote Keywhiz server.

The FUSE mount point is shared between containers if the name of the volume is the same between containers.
Otherwhise, a new volume is mounted per container.

## Usage

1. Run the daemon and connect to a Keywhiz server:

    $ go run main.go keywhiz_server_url

2. Register the plugin:

    $ echo daemon_url > /usr/share/docker/plugins/keywhiz-fs.spec

3. Run containers pointing to the driver:

    $ docker run --rm -v all-my-secrets:/etc/secrets --volume-driver keywhiz-fs -it ubuntu bash

4. :tada:

## TODO

- Allow to use a socket connection rather than tcp.
- Run inside a container.
- More TODOs.

## LICENSE

MIT
