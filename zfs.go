package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const ZPOOL = "zpool"
const ZFS = "zfs"
const MOUNT_ZFS = "mount.zfs"
const MOUNT = "mount"

const ROOT_FS = "dmfs"

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

// XXX: This might need to change when 401-node-tests lands.
func calculateMountpoint(pool, fs string) string {
	return "/var/" + pool + "/dmfs/" + fs
}

func filesystemMounted(pool, fs string) (bool, error) {
	// is filesystem mounted?
	code, err := returnCode(
		"mountpoint",
		calculateMountpoint(pool, fs),
	)
	if err != nil {
		return false, err
	}
	mounted := code == 0
	return mounted, nil
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
	args := []string{"create", "-f", pool}
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

func makeDirectoryIfNotExists(directory string) error {
	if _, err := os.Stat(directory); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(directory, 0700); err != nil {
				log.Printf("[makeDirectoryIfNotExists] error creating %s: %+v", directory, err)
				return err
			}
		} else {
			log.Printf("[makeDirectoryIfNotExists] error statting %s: %+v", directory, err)
			return err
		}
	}
	return nil
}

func mountFilesystem(pool, filesystem, mountpoint string) error {
	err := makeDirectoryIfNotExists(mountpoint)
	if err != nil {
		return err
	}
	args := []string{pool + "/" + filesystem, mountpoint}
	cmd := exec.Command(MOUNT_ZFS, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount.zfs failed (%v): %s", err, output)
	}
	return nil
}

func bindMountFilesystem(from, to string) error {
	err := makeDirectoryIfNotExists(to)
	if err != nil {
		return err
	}
	args := []string{"--bind", from, to}
	cmd := exec.Command(MOUNT, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount (bind) failed (%v): %s", err, output)
	}
	return nil
}

func returnCode(name string, arg ...string) (int, error) {
	// Run a command and either get the returncode or an error if the command
	// failed to execute, based on
	// http://stackoverflow.com/questions/10385551/get-exit-code-go
	cmd := exec.Command(name, arg...)
	if err := cmd.Start(); err != nil {
		return -1, err
	}
	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus(), nil
			}
		} else {
			return -1, err
		}
	}
	// got here, so err == nil
	return 0, nil
}
