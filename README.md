# dm-linuxkit

a oneshot dm controller for linuxkit

## design

### assumptions

* each linuxkit has zero or one dotmesh instances on it.

### behaviour

```
dm-linuxkit --storage-device=/dev/nvme0,/dev/nvme1 --dot=postgres \
    --seed=dothub.com/justincormack/postgres --mountpoint=/var/lib/postgres
```

`--dot`, `--mountpoint` and `--storage-device` are mandatory arguments

1. init zpool if not exists

  - zpool import (auto-detects zpools on block devices)
  - zpool list
  - if no zpools
    - zpool create dotmesh-pool /dev/nvme0 /dev/nvme1

2. zfs create dotmesh-pool/dotmesh-etcd
3. start an etcd process configured to write its state to /dotmesh-etcd and listen on a UNIX socket
4. start dotmesh-server configured to connect to etcd on the unix socket
5. wait for dotmesh-server to come up on :6969 (maybe it should listen on a UNIX socket!)
6. talk to the dotmesh API
7. init or pull a dot, based on config below.
8. kills dotmesh, waits for it to shut down, kills etcd, waits for it to shut down, exits.

### service

run a long-running service after the initial daemon.

```
dm-linuxkit --zpool-device=/dev/nvme0 --zpool-device=/dev/nvme1 --daemon
```

### use cases

1. create a new dot: what to call it? default to hostname? or dot=hostname. pull name from a file?
2. provision a server from a dot on the dothub. don't provision from the source dot if i already have state (reboot case).

### case 1 - seperate dots

```
dm-linuxkit --zpool-device=/dev/nvme0,/dev/nvme1 --dot=postgres \
    --seed=dothub.com/justincormack/postgres --mountpoint=/var/lib/postgres
dm-linuxkit --zpool-device=/dev/nvme0,/dev/nvme1 --dot=redis \
    --seed=dothub.com/justincormack/redis --mountpoint=/var/lib/redis
```

### case 2 - subdots
```
dm-linuxkit --zpool-device=/dev/nvme0,/dev/nvme1 --dot=myapp.postgres \
    --seed=dothub.com/justincormack/myapp --mountpoint=/var/lib/postgres
dm-linuxkit --zpool-device=/dev/nvme0,/dev/nvme1 --dot=myapp.redis \
    --seed=dothub.com/justincormack/myapp --mountpoint=/var/lib/redis
```

(second 'seed' is a no-op)

### running tests

On Ubuntu 16.04+ or macOS where you've already [installed dotmesh](https://docs.dotmesh.com/install-setup/docker/) so that the kernel module is already loaded.

```
make test
```

### future stuff

sub-cases of 2 above.

2.a) fork it, have my own timeline from that dot.

2.b) that one is me, because i've been moved.

auto-commit & push would be nice.
every new server as a branch would be nice (in the future).
