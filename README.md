# linuxkit example

You need to create a `metadata.json` file with the dotmesh hub account like this
```
{
  "dotmesh": {
     "entries": {
       "credentials": {
          "content": "username:API key"
       },
       "admin-api-key": {
          "content": "key"
       },
       "admin-password": {
          "content": "password"
       }
    }
  }
}
```

You can generate the local key and password with
```
dd if=/dev/urandom bs=1 count=32 2>/dev/null | base64
```

# dm-linuxkit
A utility for mounting dotmesh dots on a local running operating system, whether VM or bare-metal, in particular integrating with (but not requiring) [LinuxKit from Docker](https://github.com/linuxkit/linuxkit).

## Purpose
[dotmesh](https://dotmesh.com) is a system for capturing and managing snapshots of data. Combined with [dothub](https://dothub.com), it allows you to capture, store, ship, share and replay data at any given moment in time.

dotmesh itself already has native support for installing and making its fantastic capabilities available to containers running in docker and kubernetes.

This utility gives you the ability to install and run dotmesh locally, without the need for containers or orchestration systems, but still working well with them, of course.

For example, if you are running postgres locally on your server, you store all your data in a single directory, by default `/var/lib/postgres`. If you were running in kubernetes, you would use the dotmesh kubernetes volume driver and it would "just work". But you are running locally; how do you make `/var/lib/postgres` be a dot and gain all of the dot goodness, a.k.a. dotness?

Run `dm-linuxkit`.

Provide it with three options:

* Where the dot should be mounted
* Which underlying storage to use
* What to name the dot

Optionally, you even can seed it with a dot from dothub:

* What dot to use for seeding

Once run, your process will use the local directory, but you will have all of the dot benefits. 

For our above example:

```
dm-linuxkit --storage-device=/dev/nvme0,/dev/nvme1 --dot=postgres \
    --mountpoint=/var/lib/postgres
```

This will:

1. Initialize a single-node dotmesh cluster right here on our server, whether virtual or bare-metal.
2. Use storage from `/dev/nvme0` and `/dev/nvme1`
3. Create a dot called `postgres`
4. Mount the dot at `/var/lib/postgres`

If you want to seed it:

```
dm-linuxkit --storage-device=/dev/nvme0,/dev/nvme1 --dot=postgres \
    --seed=dothub.com/justincormack/postgres --mountpoint=/var/lib/postgres
```

`--remote-username` and `--remote-apikey` are necessary arguments if you pass `--seed`.

In addition to the above steps, this will seed it from the dot at `dothub.com/justincormack/postgres`.

Presto! You have dotness available on your server. No containers required!

## use on GCP

Set up your LinuxKit GCP environemnt as in [the LinuxKit GCP docs](https://github.com/linuxkit/linuxkit/blob/master/docs/platform-gcp.md).

Open the dotmesh port on GCP:
```
gcloud compute firewall-rules create "dotmesh" --allow tcp:32607 --description="Allow dotmesh access"
```

Then you can run machines with
```
linuxkit build -format gcp dotmesh.yml
linuxkit push gcp dotmesh
linuxkit run gcp -data-file metadata.json -disk size=1G dotmesh
```

You need the latest master `linuxkit` build to support metadata on GCP.

## design

### assumptions

* each linuxkit has zero or one dotmesh instances on it.

### behaviour

`--dot`, `--mountpoint` and `--storage-device` are mandatory arguments

1. init zpool if not exists

  - zpool import (auto-detects zpools on block devices)
  - zpool list
  - if no zpools
    - zpool create dotmesh-pool /dev/nvme0 /dev/nvme1

2. zfs create dotmesh-pool/dotmesh-etcd
3. start an etcd process configured to write its state to /dotmesh-etcd and listen on a UNIX socket
4. start dotmesh-server configured to connect to etcd on the unix socket
5. wait for dotmesh-server to come up on :32607 (maybe it should listen on a UNIX socket!)
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

To start dm-linuxkit in a LinuxKit VM (assuming you've installed LinuxKit):

```
make linuxkit
```

### future stuff

sub-cases of 2 above.

2.a) fork it, have my own timeline from that dot.

2.b) that one is me, because i've been moved.

auto-commit & push would be nice.
every new server as a branch would be nice (in the future).
