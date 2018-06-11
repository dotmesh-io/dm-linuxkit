TEST_POOL=test-$(shell date +%s)

disk.img:
	dd if=/dev/zero of=$@ bs=1m count=64

build: *.go
	docker build -t lmarsden/dm-linuxkit .

test: build disk.img
	# This Makefile adventure was fun, but let's move this into a go test.
	mkdir -p $(PWD)/var/dotmesh
	# - $(PWD)/var/dotmesh is where we put etcd's data dir
	# - /var/run/dotmesh is where dotmesh itself puts its mnt directory (which
	#   is where zfs filesystems get actually mounted)
	docker run -v /dev/zfs:/dev/zfs \
		--privileged \
		-v $(PWD)/var/dotmesh:$(PWD)/var/dotmesh:rshared \
		-v /var/run/dotmesh:/var/run/dotmesh:rshared \
		-v $(PWD)/disk.img:$(PWD)/disk.img lmarsden/dm-linuxkit \
		dm-linuxkit -dot=test -storage-device=$(PWD)/disk.img \
			-mountpoint=/tmp/test -oneshot -pool-name=$(TEST_POOL) || true
	docker run -v /dev/zfs:/dev/zfs \
		--privileged -v $(PWD)/disk.img:$(PWD)/disk.img lmarsden/dm-linuxkit \
		zpool destroy $(TEST_POOL)

linuxkit: build
	linuxkit build dotmesh.yml
	rm -rf dotmesh-state
	linuxkit run qemu -accel tcg -publish 32607:32607 -data-file metadata.json -disk size=1024M -mem 2048 dotmesh

build-and-push-gcp: build
	linuxkit build -format gcp dotmesh.yml
	linuxkit push gcp dotmesh.img.tar.gz
	echo "now run:"
	echo "    linuxkit run gcp -data-file metadata.json -disk size=1G -name dotmesh0 dotmesh"
	echo "or:"
	echo "    linuxkit run gcp -data-file metadata-seed.json -disk size=1G -name dotmesh1 dotmesh"

get-dotmesh0-ip:
	gcloud compute instances describe --format json dotmesh0 | jq .networkInterfaces[0].accessConfigs[0].natIP |tr -d '"'

get-dotmesh1-ip:
	gcloud compute instances describe --format json dotmesh1 | jq .networkInterfaces[0].accessConfigs[0].natIP |tr -d '"'

build-jenkins:
	linuxkit build -format gcp jenkins.yml
	linuxkit push gcp jenkins.img.tar.gz
	linuxkit build jenkins.yml

run-jenkins-gcp:
	linuxkit run gcp -machine n1-standard-1 -data-file metadata.json -disk size=1G jenkins

clean:
	rm -rf *-state *-kernel *-cmdline *-initrd.img *.iso
