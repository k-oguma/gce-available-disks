package src

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/patrickmn/go-cache"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/api/compute/v1"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"syscall"
	"time"
)

func Inquire(ctx context.Context, something string, projects []string, computeService *compute.Service, Opts map[string]interface{}) []string {

	var results []string
	c := &LogConfigure{
		IterCache:     cache.New(30*time.Minute, 60*time.Minute),
		maxLogEntries: 5000,
		timeout: time.Duration(Opts["timeout"].(int)) * time.Second,
		isActivityLog: Opts["activityLog"].(bool),
		isCSV:	Opts["csvMode"].(bool),
		isSilent:	Opts["silentMode"].(bool),
	}

	// By default 900 sec, if you set the timeout option, the isActivityLog option is also enabled in conjunction.
	if c.timeout != time.Duration(900) * time.Second {
		c.isActivityLog = true
	}

	if c.isCSV { c.writer = csv.NewWriter(os.Stdout) }

	switch something {
	case "availableDisk":
		for _, project := range projects {
			c.projID = project
			var schemas []string
			c.requireKeys = []string{
				"sizeGb",
				"creationTimestamp",
			}
			c.parentKey = "disks"

			if c.isCSV {
				for _, schema := range []string{"project", "available disk name"} {
					schemas = append(schemas, schema)
				}

				for _, schema := range c.requireKeys {
					schemas = append(schemas, schema)
				}

				if c.isActivityLog {
					schemas = append(schemas, "createdBy")
				}

				c.writer.Write(schemas)
			}

			for _, result := range c.findAvailableDisks(ctx, computeService) {
				results = append(results, result)
			}
		}
	//case something:
	default:
	}
	ctx.Done()
	return results
}

func (c *LogConfigure) getActivityLog(diskInfo []string) string {
	var activityLog string
	if len(diskInfo) != 0 {
		var err error

		// Get audit log
		if activityLog, err = c.ActivityLog(); err != nil {
			log.Fatalln(err)
		}
	 }
	return activityLog
}

func showUser(user string) string {
	return fmt.Sprintf("\t\x1b[1;37mcreatedBy:\x1b[0m %s\n", user)
}

func readLine() string {
	var (
		bufSize = 10000
		rdr     = bufio.NewReaderSize(os.Stdin, bufSize)
	)

	buf := make([]byte, 0, bufSize)
	for {
		l, p, err := rdr.ReadLine()
		if err != nil {
			panic(err)
		}
		buf = append(buf, l...)
		if !p {
			break
		}
	}
	return string(buf)
}

func EnsureProjects() []string {
	if reader := bufio.NewReader(os.Stdin); reader != nil {
		var output []rune

		for {
			input, _, err := reader.ReadRune()
			if err != nil && err == io.EOF {
				break
			}
			output = append(output, input)
		}
		return strings.Split(strings.TrimSuffix(string(output),"\n")," ")
	}

	if terminal.IsTerminal(syscall.Stdin) {
		fmt.Printf("\x1b[36m%s:\x1b[39m\n\n%s\n\n", "Please enter "+
			""+
			"your GCP projects", "e.g. xxxxxx-sre-dev xxxxxx-sre-prd")

		stdin := readLine()

		return strings.Split(stdin, " ")
	}

	body, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}

	return strings.Split(string(body), " ")
}

func (c *LogConfigure) findAvailableDisks(ctx context.Context, computeService *compute.Service) []string {
	var diskInfo []string

	req := computeService.Disks.AggregatedList(c.projID)

	if err := req.Pages(ctx, func(page *compute.DiskAggregatedList) error {
		for _, disksScopedList := range page.Items {
			if disksScopedList.Disks != nil {
				dsl, err := disksScopedList.MarshalJSON()
				if err != nil {
					fmt.Println("error:", err)
				}

				var jsonDecode interface{}
				err = json.Unmarshal(dsl, &jsonDecode)
				if err != nil {
					fmt.Println("error:", err)
				}

				diskInfo = c.confirmInUse(&disksScopedList, &jsonDecode, diskInfo)
			}
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}
	return diskInfo
}

// Normal output style
func (c *LogConfigure) outputYamlStyle(jsonDecode *interface{}, diskInfo []string, activityLog, isType string, i int) ([]string, error) {
	switch isType {
	case "disk attribute":
		diskInfo = append(diskInfo, fmt.Sprintf(
			"project: %s\n", c.projID))

		diskInfo = append(diskInfo, fmt.Sprintf(
			"\t\x1b[36mavailable disk name:\x1b[0m %s\n", c.diskName))

		for index, key := range c.requireKeys {
			diskInfo = append(diskInfo, fmt.Sprintf(
				"\t\x1b[32m%s:\x1b[0m  %s\n", c.requireKeys[index], jsonExtractValue(*jsonDecode, c.parentKey, i, key)))
		}

		return diskInfo, nil

	case "owner":
		diskInfo = append(diskInfo, showUser(activityLog))
		return diskInfo, nil

	default:
		log.SetPrefix("outputYamlStyle: ")

	}

	log.Println("Type no match.")
	log.Printf("Construct disk information. diskInfo is '%s' ", diskInfo)
	return diskInfo, fmt.Errorf("Disk information error.")
}

func (c *LogConfigure) outputCSVStyle(jsonDecode *interface{}, diskInfo []string, activityLog, isType string, i int) ([][]string, error) {
	var diskInfos [][]string

	switch isType {
	case "disk attribute":
		diskInfo = append(diskInfo, c.projID)
		diskInfo = append(diskInfo, c.diskName)

		for _, key := range c.requireKeys {
			diskInfo = append(diskInfo, fmt.Sprintf("%s", jsonExtractValue(*jsonDecode, c.parentKey, i, key)))
		}
		diskInfos = append(diskInfos, diskInfo)
		return diskInfos, nil
	//case "owner":

	default:
		log.SetPrefix("outputCSVStyle: ")
	}

	log.Println("Type no match.")
	log.Printf("Construct disk information. diskInfo is '%s' ", diskInfo)
	return diskInfos, fmt.Errorf("Disk information error.")
}

// If users is nil and READY of also status then available disks.
func (c *LogConfigure) confirmInUse(dsl *compute.DisksScopedList, jsonDecode *interface{}, diskInfo []string) []string {
	var err error
	var diskInfos [][]string

	for i := range dsl.Disks {
		if jsonExtractValue(*jsonDecode, c.parentKey, i, "users") == nil {
			if jsonExtractValue(*jsonDecode, c.parentKey, i, "status") == "READY" {
				c.diskName = fmt.Sprintf("%s", jsonExtractValue(*jsonDecode, c.parentKey, i, "name"))

				if c.isCSV {
					diskInfos, err = c.outputCSVStyle(jsonDecode, diskInfo,"", "disk attribute", i)
				} else {
					// normal output mode. like a YAML
					diskInfo, err = c.outputYamlStyle(jsonDecode, diskInfo,"", "disk attribute", i)
				}

				if err != nil {
					log.Fatalln(err)
				}

				// Get activity logs by Stackdriver Logging and check disk creator.
				if c.isActivityLog {
					ch := make(chan string, 10)
					go func() {
						defer close(ch)

						var (
							activityLog string
							err error
						)

						activityLog, err = c.ActivityLog()
						if err != nil {
							log.Fatalln(err)
						}

						ch <- activityLog
						return
					}()

					select {
					case activityLog := <-ch:

						if !strings.EqualFold(activityLog, "") {
							if c.isCSV {
								diskInfos[i] = append(diskInfos[i], activityLog)
							} else {
								diskInfo, err = c.outputYamlStyle(jsonDecode, diskInfo, activityLog, "owner", i)
							}
						}

					case <-time.After(c.timeout):
						// call timed out
						if !c.isSilent {
							log.Printf(`ActiveLog get timeout. Over %d second.
								Probably, %s has fairly large activity logs.
								However, could not be obtained only activity logs from Stackdriver in the GCP project.
								It is obtaining other disk information.

								Please make sure Google Cloud Console: https://console.cloud.google.com/logs/`,
								c.timeout / 1000000000, c.projID)
						}
					}
				}
			}
		}
	}
	if c.isCSV { c.writer.WriteAll(diskInfos) }
	return diskInfo
}

func jsonExtractValue(json interface{}, parentKey string, index int, key string) interface{} {
	return json.(map[string]interface{})[parentKey].([]interface{})[index].(map[string]interface{})[key]
}
