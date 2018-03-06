package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const ZPOOL = "zpool"
const ZFS = "zfs"

// TODO dedupe wrt dotmesh's zfs.go
func findLocalPoolId(pool string) (string, error) {
	output, err := exec.Command(ZPOOL, "get", "-H", "guid", pool).CombinedOutput()
	if err != nil {
		return string(output), err
	}
	i, err := strconv.ParseUint(strings.Split(string(output), "\t")[2], 10, 64)
	if err != nil {
		return string(output), err
	}
	return fmt.Sprintf("%x", i), nil
}

func createPool(pool string, devices []string) error {
	// create the pool
	args := []string{"create", pool}
	args = append(args, devices...)
	cmd := exec.Command(ZPOOL, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("zpool create failed (%v): %s", err, output)
	}
	return nil
}

func createFilesystem(filesystem, pool string) error {
	// TODO: there's no automounter in LinuxKit, so we probably want to use
	// mountpoint=legacy and just call mount.zfs
	args := []string{"create", "-o", "mountpoint=" + ETCD_DATA_DIR, pool + "/" + filesystem}
	cmd := exec.Command(ZFS, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("zfs create failed (%v): %s", err, output)
	}
	return nil
}
