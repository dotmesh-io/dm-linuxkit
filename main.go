package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const ETCD_DATA_DIR = "/var/dotmesh/etcd"
const ETCD_ENDPOINT = "http://localhost:2379"
const RPC_TIMEOUT = 1 * time.Minute

// TODO: factor this out. https://github.com/dotmesh-io/dotmesh/issues/44

type TransferRequest struct {
	Peer             string
	User             string
	ApiKey           string
	Direction        string
	LocalNamespace   string
	LocalName        string
	LocalBranchName  string
	RemoteNamespace  string
	RemoteName       string
	RemoteBranchName string
	TargetCommit     string
}

type TransferPollResult struct {
	TransferRequestId string
	Peer              string // hostname
	User              string
	ApiKey            string // protected value in toString()
	Direction         string // "push" or "pull"

	// Hold onto this information, it might become useful for e.g. recursive
	// receives of clone filesystems.
	LocalFilesystemName  string
	LocalCloneName       string
	RemoteFilesystemName string
	RemoteCloneName      string

	// Same across both clusters
	FilesystemId string

	InitiatorNodeId string
	PeerNodeId      string

	StartingSnapshot string
	TargetSnapshot   string

	Index              int    // i.e. transfer 1/4 (Index=1)
	Total              int    //                   (Total=4)
	Status             string // one of "starting", "running", "finished", "error"
	NanosecondsElapsed int64
	Size               int64 // size of current segment in bytes
	Sent               int64 // number of bytes of current segment sent so far
	Message            string
}

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
	// TODO: Add a flag for specifying a branch of a dot, rather than always
	// defaulting to master.
	flagMountpoint := flag.String(
		"mountpoint", "",
		"Where to mount the datadot on the host",
	)
	flagSeed := flag.String(
		"seed", "",
		"Address of a datadot to seed from e.g. dothub.com/justincormack/postgres",
	)
	flagOneShot := flag.Bool(
		"oneshot", false,
		"Exit immediately, useful for initializing things on boot. "+
			"Otherwise, runs as long-running daemon to support e.g. dm CLI interactions",
	)
	flagCredentialsFile := flag.String(
		"credentials-file", "/run/config/dotmesh/credentials",
		"File containing <API username>:<API key> for use with -seed",
	)
	flagAdminApiKeyFile := flag.String(
		"admin-api-key-file", "/run/config/dotmesh/admin-api-key",
		"Initial admin API key for the local dotmesh",
	)
	flagAdminPasswordFile := flag.String(
		"admin-password-file", "/run/config/dotmesh/admin-password",
		"Initial admin password for the local dotmesh",
	)
	flag.Parse()

	err := setupZFS(*flagPool, strings.Split(*flagStorageDevice, ","))
	if err != nil {
		panic(err)
	}

	etcdCmd, err := runEtcd(*flagPool)
	if err != nil {
		panic(err)
	}

	adminPasswordBytes, err := ioutil.ReadFile(*flagAdminPasswordFile)
	if err != nil {
		panic(err)
	}

	adminPassword := string(adminPasswordBytes)

	adminApiKeyBytes, err := ioutil.ReadFile(*flagAdminApiKeyFile)
	if err != nil {
		panic(err)
	}

	adminApiKey := string(adminApiKeyBytes)

	dotmeshCmd, err := runDotmesh(*flagPool, adminPassword, adminApiKey)
	if err != nil {
		panic(err)
	}

	// TODO: wait for dotmesh to start, make a dot.

	var result bool
	var resultString string

	for {
		err := doRPC("localhost", "admin", adminApiKey, "DotmeshRPC.Ping", nil, &result)
		if err == nil {
			log.Printf("Connected! Yay!")
			break
		}
		log.Printf("Error, retrying... %v", err)
		time.Sleep(1 * time.Second)
	}

	if *flagSeed != "" {
		// Extract api username and key from environment metadata.
		credentialsBytes, err := ioutil.ReadFile(*flagCredentialsFile)
		if err != nil {
			log.Printf(
				"Unable to read credentials file at %s, see the "+
					"README for how to provide credentials for seeding.",
				*flagCredentialsFile,
			)
			panic(err)
		}
		// XXX handle :s in the username
		shrapnel := strings.Split(string(credentialsBytes), ":")
		username := shrapnel[0]
		apiKey := shrapnel[1]
		log.Printf("got username=%s, apiKey=%s", username, apiKey)

		// TODO interpret 'dothub.com/justincormack/postgres'
		shrapnel = strings.Split(*flagSeed, "/")
		if len(shrapnel) != 3 {
			panic(fmt.Errorf(
				"Need exactly two '/'s in -seed argument, " +
					"e.g. 'dothub.com/justincormack/postgres'",
			))
		}
		hostname := shrapnel[0]        // dothub.com
		remoteNamespace := shrapnel[1] // e.g. justincormack
		remoteName := shrapnel[2]      // e.g. postgres

		var transferId string
		err = doRPC(
			"localhost", "admin", adminApiKey,
			"DotmeshRPC.Transfer", TransferRequest{
				Peer:             hostname,
				User:             username,
				ApiKey:           apiKey,
				Direction:        "pull",
				LocalNamespace:   "admin",
				LocalName:        *flagDot,
				LocalBranchName:  "",
				RemoteNamespace:  remoteNamespace,
				RemoteName:       remoteName,
				RemoteBranchName: "",
			}, &transferId)
		if err != nil {
			panic(err)
		}

		err = func() error {
			started := false
			debugMode := true

			for {
				if debugMode {
					log.Printf("DEBUG About to sleep for 1s...")
				}
				time.Sleep(time.Second)
				result := &TransferPollResult{}

				if debugMode {
					log.Printf("DEBUG Calling GetTransfer(%s)...", transferId)
				}
				err := doRPC(
					"localhost", "admin", adminApiKey,
					"DotmeshRPC.GetTransfer", transferId, result,
				)
				if debugMode {
					log.Printf(
						"DEBUG done GetTransfer(%s), got err %#v and result %#v...",
						transferId, err, result,
					)
				}
				if debugMode {
					log.Printf("DEBUG rpcError consumed!")
				}

				if debugMode {
					log.Printf("DEBUG Got err: %s", err)
				}
				if err != nil {
					if !strings.Contains(fmt.Sprintf("%s", err), "No such intercluster transfer") {
						log.Printf("Got error, trying again: %s", err)
					}
				}

				if debugMode {
					log.Printf("Got DotmeshRPC.GetTransfer response: %+v", result)
				}
				if !started {
					log.Printf("Starting transfer of %d bytes...", result.Size)
					started = true
				}
				log.Printf(result.Status)
				var speed string
				if result.NanosecondsElapsed > 0 {
					speed = fmt.Sprintf(" %.2f MiB/s",
						// mib/sec
						(float64(result.Sent)/(1024*1024))/
							(float64(result.NanosecondsElapsed)/(1000*1000*1000)),
					)
				} else {
					speed = " ? MiB/s"
				}
				quotient := fmt.Sprintf(" (%d/%d)", result.Index, result.Total)
				log.Printf(speed + quotient)

				if result.Index == result.Total && result.Status == "finished" {
					if started {
						log.Printf("Done!")
					}
					time.Sleep(time.Second)
					return nil
				}
				if result.Status == "error" {
					if started {
						log.Printf("error: %s", result.Message)
					}
					time.Sleep(time.Second)
					return fmt.Errorf(result.Message)
				}
			}
		}()
		if err != nil {
			panic(err)
		}

	} else {
		// see if the dot already exists
		if err := tryUntilSucceedsN(func() error {
			return doRPC(
				"localhost", "admin", adminApiKey,
				"DotmeshRPC.Exists",
				map[string]string{"Name": *flagDot, "Namespace": "admin"},
				&resultString,
			)
		}, fmt.Sprintf("check if %s exists", *flagDot), 5); err != nil {
			panic(err)
		}
		// create if does not exist
		if resultString == "" {
			if err := doRPC(
				"localhost", "admin", adminApiKey,
				"DotmeshRPC.Create",
				map[string]string{"Name": *flagDot, "Namespace": "admin"},
				&result,
			); err != nil {
				panic(err)
			}
			log.Printf("Created dot %s!", *flagDot)
		} else {
			log.Printf("Found existing dot %s!", *flagDot)
		}
	}

	// TODO: mount the dot on the filesystem at flagMountpoint, after doing
	// mkdir flagMountpoint

	// Find the ID of the dot.
	var lookupResult string
	err = doRPC(
		"localhost", "admin", adminApiKey,
		"DotmeshRPC.Lookup",
		map[string]string{"Name": *flagDot, "Namespace": "admin"},
		&lookupResult,
	)
	if err != nil {
		panic(err)
	}

	// TODO: switch this to running DotmeshRPC.Procure once that yields actual
	// mount points, rather than symlinks.
	// Related: https://github.com/dotmesh-io/dotmesh/issues/421

	err = bindMountFilesystem(
		// Seems like this MOUNT_PREFIX of /var is set in dotmesh utils.go
		// (unless MOUNT_PREFIX is set, and we're not setting it...)
		calculateMountpoint(*flagPool, lookupResult),
		*flagMountpoint,
	)
	if err != nil {
		panic(err)
	}

	// SHUTDOWN FOLLOWS

	if *flagOneShot {
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
	} else {
		err = dotmeshCmd.Wait()
		if err != nil {
			log.Printf("dotmesh exited with %v, this is unusual (we were in non-oneshot mode)", err)
		}
		err = etcdCmd.Wait()
		if err != nil {
			log.Printf("etcd exited with %v, this is unusual (we were in non-oneshot mode)", err)
		}
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
	exists, err := filesystemExists(pool, "dotmesh-etcd")
	if err != nil {
		panic(err)
	}
	if !exists {
		err := createFilesystem(pool, "dotmesh-etcd")
		if err != nil {
			return nil, err
		}
	}
	mounted, err := filesystemMounted(ETCD_DATA_DIR)
	if err != nil {
		panic(err)
	}
	if !mounted {
		err = mountFilesystem(pool, "dotmesh-etcd", ETCD_DATA_DIR)
		if err != nil {
			return nil, err
		}
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
		"CONTAINER_RUNTIME=null",                            // disables docker integration
		"MOUNT_PREFIX=/var/dotmesh/mnt",                     // must be set in newer dotmeshes
		"CONTAINER_MOUNT_PREFIX=/var/dotmesh/container_mnt", // must be set in newer dotmeshes
		"DISABLE_FLEXVOLUME=1",                              // don't install kubernetes driver
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
