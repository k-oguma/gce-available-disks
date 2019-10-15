package main

// BEFORE RUNNING:
// ---------------
// 1. If not already done, enable the Compute Engine API
//    and check the quota for your project at
//    https://console.developers.google.com/apis/api/compute
// 2. This sample uses Application Default Credentials for authentication.
//    If not already done, install the gcloud CLI from
//    https://cloud.google.com/sdk/ and run
//    `gcloud beta auth application-default login`.
//    For more information, see
//    https://developers.google.com/identity/protocols/application-default-credentials
// 3. Install and update the Go dependencies by running `go get -u` in the
//    project directory.

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/api/option"

	"github.com/k-oguma/gce-available-disks/src"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

var (
	timeoutOpt     = flag.Int("t", 900, "Time out for get activity log from Stackdriver logging. So this option when specified is also -l option enabled. \ne.g.) -t 120 <GCP_project>")
	activityLogOpt = flag.Bool("l", false, "Get activity log for the disk. You able to know creator of the disk. (Default is false)")
	csvModeOpt     = flag.Bool("c", false, "CSV output mode. (Default is false)")
	silentModeOpt  = flag.Bool("s", false, "Silent mode. (Default is false)")
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()

	var Opts = map[string]interface{}{
		"timeout":     *timeoutOpt,
		"activityLog": *activityLogOpt,
		"csvMode":     *csvModeOpt,
		"silentMode":  *silentModeOpt,
	}

	var projects []string
	projects = flag.Args()

	if len(os.Getenv("PROJECT")) != 0 {
		projects = append(projects, os.Getenv("PROJECT"))
	}

	if len(projects) == 0 {
		projects = src.EnsureProjects()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	gdc, err := google.DefaultClient(ctx, compute.CloudPlatformScope)
	if err != nil {
		log.Fatal(err)
	}

	computeService, err := compute.NewService(ctx, option.WithHTTPClient(gdc))
	if err != nil {
		log.Fatal(err)
	}
	diskInfo := src.Inquire(ctx, "availableDisk", projects, computeService, Opts)

	for _, name := range diskInfo {
		fmt.Print(name)
	}
}
