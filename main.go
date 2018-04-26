package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const ETCD_DATA_DIR = "/var/dotmesh/etcd"
const ETCD_ENDPOINT = "http://localhost:2379"

func main() {
	flagStorageDevice := flag.String(
		"storage-device", "",
		"block device or file to store data (seperate multiple with commas)",
	)
	flagPool := flag.String(
		"pool-name", "pool",
		"Name of storage pool to use",
	)
	flagDot := flag.String(
		"dot", "",
		"Name of dotmesh datadot to use (docs.dotmesh.com/concepts/what-is-a-datadot)",
	)
	flagMountpoint := flag.String(
		"mountpoint", "",
		"Where to mount the datadot on the host",
	)
	flagSeed := flag.String(
		"seed", "",
		"Address of a datadot to seed from e.g. dothub.com/justincormack/postgres",
	)
	flagDaemon := flag.Bool(
		"oneshot", false,
		"Exit immediately, useful for initializing things on boot. "+
			"Otherwise, runs as long-running daemon to support e.g. dm CLI interactions",
	)
	flag.Parse()

	log.Printf(
		"%s %s %s %s %s %b",
		*flagStorageDevice, *flagPool, *flagDot, *flagMountpoint, *flagSeed, *flagDaemon,
	)

	err := setupZFS(*flagPool, strings.Split(*flagStorageDevice, ","))
	if err != nil {
		panic(err)
	}

	etcdCmd, err := runEtcd(*flagPool)
	if err != nil {
		panic(err)
	}

	adminPassword, err := GenerateRandomString(16)
	if err != nil {
		panic(err)
	}
	adminApiKey, err := GenerateRandomString(16)
	if err != nil {
		panic(err)
	}

	dotmeshCmd, err := runDotmesh(*flagPool, adminPassword, adminApiKey)
	if err != nil {
		panic(err)
	}

	// TODO: wait for dotmesh to start, make a dot.

	log.Printf("Admin API key is: %s", adminApiKey)

	var result bool

	for {
		err := doRPC("localhost", "admin", adminApiKey, "DotmeshRPC.Ping", nil, &result)
		if err == nil {
			log.Printf("Connected! Yay!")
			break
		}
		log.Printf("Error, retrying... %v", err)
		time.Sleep(1 * time.Second)
	}

	err = doRPC(
		"localhost", "admin", adminApiKey,
		"DotmeshRPC.Create",
		map[string]string{"Name": *flagDot, "Namespace": "admin"},
		&result,
	)
	if err != nil {
		panic(err)
	}
	log.Printf("Created dot %s!", *flagDot)

	err = dotmeshCmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		panic(err)
	}

	err = dotmeshCmd.Wait()
	if err != nil {
		log.Printf("dotmesh exited with %v, this is normal (we just killed it)", err)
	}

	err = etcdCmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		panic(err)
	}

	err = etcdCmd.Wait()
	if err != nil {
		log.Printf("etcd exited with %v, this is be normal (we just killed it)", err)
	}

}

func setupZFS(pool string, devices []string) error {

	_, err := findLocalPoolId(pool)
	if err == nil {
		// pool already exists
		return nil
	}

	return createPool(pool, devices)

}

func runEtcd(pool string) (*exec.Cmd, error) {
	// 1. create a zfs filesystem for etcd if it doesn't exist already
	err := createFilesystem("dotmesh-etcd", pool)
	if err != nil {
		return nil, err
	}
	// 2. start etcd
	cmd := exec.Command("etcd",
		"-data-dir", ETCD_DATA_DIR,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

func runDotmesh(pool, adminPassword, adminApiKey string) (*exec.Cmd, error) {
	adminPasswordBase64 := base64.StdEncoding.EncodeToString([]byte(adminPassword))
	adminApiKeyBase64 := base64.StdEncoding.EncodeToString([]byte(adminApiKey))

	cmd := exec.Command("dotmesh-server")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		// TODO: disable docker volume plugin
		// TODO: disable docker integration
		"DISABLE_FLEXVOLUME=1",
		fmt.Sprintf("DOTMESH_ETCD_ENDPOINT=%s", ETCD_ENDPOINT),
		fmt.Sprintf("POOL=%s", pool),
		fmt.Sprintf("INITIAL_ADMIN_API_KEY=%s", adminApiKeyBase64),
		fmt.Sprintf("INITIAL_ADMIN_PASSWORD=%s", adminPasswordBase64),
	)
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	return cmd, nil
}
