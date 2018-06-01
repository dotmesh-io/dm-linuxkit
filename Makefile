TEST_POOL=test-$(shell date +%s)

disk.img:
	dd if=/dev/zero of=$@ bs=1M count=64

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
		-v $(PWD)/disk.img:$(PWD)/disk.img dm-linuxkit \
		dm-linuxkit -dot=test -storage-device=$(PWD)/disk.img \
			-mountpoint=/tmp/test -oneshot -pool-name=$(TEST_POOL) || true
	docker run -v /dev/zfs:/dev/zfs \
		--privileged -v $(PWD)/disk.img:$(PWD)/disk.img dm-linuxkit \
		zpool destroy $(TEST_POOL)

linuxkit: build
	linuxkit build dotmesh.yml
	rm -rf dotmesh-state
	linuxkit run qemu -accel tcg -data-file metadata.json -disk size=1024M -mem 2048 dotmesh
