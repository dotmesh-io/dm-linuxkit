package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const ZPOOL = "zpool"
const ZFS = "zfs"
const MOUNT_ZFS = "mount.zfs"

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

func filesystemExists(pool, filesystem string) (bool, error) {
	cmd := exec.Command(ZFS, "list", pool+"/"+filesystem)
	if err := cmd.Start(); err != nil {
		return false, err
	}
	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus() == 0, nil
			}
		} else {
			return false, err
		}
	}
	// No error, filesystem exists
	return true, nil
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

func createFilesystem(pool, filesystem string) error {
	// TODO: there's no automounter in LinuxKit, so we probably want to use
	// mountpoint=legacy and just call mount.zfs
	args := []string{"create", "-o", "mountpoint=legacy", pool + "/" + filesystem}
	cmd := exec.Command(ZFS, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("zfs create failed (%v): %s", err, output)
	}
	return nil
}

func mountFilesystem(pool, filesystem, mountpoint string) error {
	args := []string{pool + "/" + filesystem, mountpoint}
	cmd := exec.Command(MOUNT_ZFS, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount.zfs failed (%v): %s", err, output)
	}
	return nil
}
