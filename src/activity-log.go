package src

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"

	"cloud.google.com/go/logging"
	"cloud.google.com/go/logging/logadmin"
	"github.com/patrickmn/go-cache"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/status"
)

var client *logadmin.Client

// ActivityLog return user name of disk creator from the Stackdriver admin logging.
func (c *LogConfigure) ActivityLog() (string, error) {
	ctx := context.Background()
	var err error
	client, err = logadmin.NewClient(ctx, c.projID)
	if err != nil {
		log.Fatalf("creating logging client: %v", err)
	}

	user, err := c.getEntries("User")
	if err != nil {
		return user, fmt.Errorf("could not get entries: %v", err)
	}
	return user, nil
}

func (c *LogConfigure) getEntries(pbstruct string) (string, error) {
	ctx := context.Background()
	//const name = "compute.googleapis.com%2Factivity_log" // Don't use this because it default is short lifetime.
	const name = "cloudaudit.googleapis.com%2Factivity"
	var (
		ic      interface{}
		found   bool
		iter    *logadmin.EntryIterator
		payload *ActivityPayload
		pau     string
		err     error
		entry   *logging.Entry
	)

	if ic, found = c.IterCache.Get(c.projID); !found {
		iter = client.Entries(ctx, logadmin.Filter(fmt.Sprintf("logName = projects/%s/logs/%s protoPayload.request.@type = type.googleapis.com/compute.disks.insert operation.last = true", c.projID, name)))

		c.IterCache.Set(c.projID, iter, cache.DefaultExpiration)

		if ic, found = c.IterCache.Get(c.projID); !found {
			log.Fatalln("Cache error")
		}
	}

	for i := 0; i < c.maxLogEntries; i++ {
		entry, err = ic.(*logadmin.EntryIterator).Next()
		if err == iterator.Done {
			break

		} else if errStatus, _ := status.FromError(err); errStatus.Message() == "grpc: the client connection is closing" {

			if !c.isSilent {
				fmt.Printf("\x1b[31m%s\n\x1b[35mRetry get logging from the Stackdriver.(not using cache)\x1b[39m\n", errStatus.Err())
				fmt.Println("\x1b[36mIf this message is noisy, you can use -s option.\x1b[39m")
			}

			// Retry direct access the Stackdriver.
			iter = client.Entries(ctx, logadmin.Filter(fmt.Sprintf("logName = projects/%s/logs/%s protoPayload.request.@type = type.googleapis.com/compute.disks.insert operation.last = true", c.projID, name)))
			entry, err = iter.Next()

		} else if err != nil {
			return "", err
		}

		bytes, err := json.Marshal(&entry.Payload)

		if err != nil {
			log.Fatal("marshaling error: ", err)
		}

		if err := json.Unmarshal(bytes, &payload); err != nil {
			log.Fatal("unmarshaling error: ", err)
		}

		if regexp.MustCompile("projects/.*/disks/(.*)").ReplaceAllString(payload.ResourceName, "$1") == c.diskName {
			pau = payload.AuthenticationInfo.PrincipalEmail
			break
		} else {
			pau = "Can't get user information. Maybe, the Stackdriver logging expired. See also: https://cloud.google.com/logging///quotas?#logs_retention_periods or https://cloud.google.com/logging///docs/audit/#audit_log_retention"
		}
	}

	if err := client.Close(); err != nil {
		log.Fatalf("Failed to close client: %v", err)
	}

	return pau, nil
}
