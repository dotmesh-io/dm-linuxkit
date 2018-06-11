## create data on one node, load it on another

Set up an `env.sh` as follows:
```
export CLOUDSDK_CORE_PROJECT=dotmesh-demos
export CLOUDSDK_COMPUTE_ZONE=europe-west1-d
export CLOUDSDK_IMAGE_BUCKET=dotmesh-linuxkit-demo
```

```
source env.sh
```

Write `metadata.json`:
```
{
  "dotmesh": {
     "entries": {
       "credentials": {
          "content": "<dothub-username>:<api-key>"
       },
       "admin-api-key": {
          "content": "<local-random-api-key>"
       },
       "admin-password": {
          "content": "<local-random-password>"
       }
    }
  }
}
```

```
gcloud compute firewall-rules create "dotmesh" --allow tcp:32607 --description="Allow dotmesh access"
```

```
make build-and-push-gcp
```

In another terminal:
```
linuxkit run gcp -data-file metadata.json -disk size=1G -name dotmesh0 dotmesh
```

Back on the first:

```
ADMIN_API_KEY=$(
    cat metadata.json |
    jq '.dotmesh.entries["admin-api-key"].content' |tr -d '"'
)
```
```
DOTMESH0_IP=$(
    gcloud compute instances describe --format json dotmesh0 |
    jq .networkInterfaces[0].accessConfigs[0].natIP |tr -d '"'
)
```
```
echo $ADMIN_API_KEY |dm remote add linuxkit-0 admin@$DOTMESH0_IP
```
```
dm list
```

On your dotmesh0 serial console:
```
cd /var/dot/test
echo "IT WORKS" > HELLO
```

Back on your first terminal:

```
dm commit -m "It works commit."
dm push hub test
```

This assumes you've got an account set up on [dothub](https://dothub.com/) and configured your `dm` client with it.
Go and check that the commit made it to the [dothub](https://dothub.com/).

Now you can seed a new LinuxKit from the first:

Write `metadata-seed.json` (`<dot-name>` is `test` in the `dotmesh.yml`):
```
{
  "dotmesh": {
     "entries": {
       "seed": {
          "content": "dothub.com/<dothub-username>/<dot-name>"
       },
       "credentials": {
          "content": "<dothub-username>:<api-key>"
       },
       "admin-api-key": {
          "content": "<local-random-api-key>"
       },
       "admin-password": {
          "content": "<local-random-password>"
       }
    }
  }
}
```

In a third terminal:
```
linuxkit run gcp -data-file metadata-seed.json -disk size=1G -name dotmesh1 dotmesh
```

```
DOTMESH1_IP=$(
    gcloud compute instances describe --format json dotmesh1 |
    jq .networkInterfaces[0].accessConfigs[0].natIP |tr -d '"'
)
```
```
echo $ADMIN_API_KEY |dm remote add linuxkit-1 admin@$DOTMESH1_IP
```
```
dm list
```

You should see it show up, and also, any app you're running on the second LinuxKit should be seeded with the data from the first!
