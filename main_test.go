package main

import (
	"context"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/k-oguma/gce-available-disks/src"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

var (
	testTimeoutOpt     = flag.Int("tt", 900, "Time out for get activity log from Stackdriver logging. So this option when //specified is also -l option enabled. \ne.g.) -t 120 <GCP_project>")
	testActivityLogOpt = flag.Bool("tl", false, "Get activity log for the disk. You able to know creator of the disk. //(Default is false)")
	testCsvModeOpt     = flag.Bool("tc", false, "CSV output mode. (Default is false)")
	testSilentModeOpt  = flag.Bool("ts", false, "Silent mode. (Default is false)")
)

func TestAvailableDiskCheck(t *testing.T) {
	flag.Parse()

	var Opts = map[string]interface{}{
		"timeout":     *testTimeoutOpt,
		"activityLog": *testActivityLogOpt,
		"csvMode":     *testCsvModeOpt,
		"silentMode":  *testSilentModeOpt,
	}

	var projects []string
	projects = flag.Args()

	if len(os.Getenv("PROJECT")) >= 1 {
		projects = append(projects, os.Getenv("PROJECT"))
	}

	if len(projects) == 0 {
		projects = src.EnsureProjects()
	}

	ctx := context.Background()
	c, err := google.DefaultClient(ctx, compute.CloudPlatformScope)
	if err != nil {
		t.Fatal(err)
	}

	computeService, err := compute.New(c)
	if err != nil {
		t.Fatal(err)
	}

	text := make(map[int]string, 5)
	text[0] = "project:"
	text[1] = "available disk name:"
	text[2] = "sizeGb:"
	text[3] = "creationTimestamp:"
	text[4] = "createdBy:"

	diskInfo := src.Inquire(ctx, "availableDisk", projects, computeService, Opts)

	for _, str := range diskInfo {
		if strings.Contains(str, text[0]) ||
			strings.Contains(str, text[1]) ||
			strings.Contains(str, text[2]) ||
			strings.Contains(str, text[3]) ||
			strings.Contains(str, text[4]) {
			t.Logf("%s is OK\n", str)
		} else {
			t.Errorf("%s is NG\n", str)
		}
	}
}
