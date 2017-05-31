package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hpcloud/tail"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "deephealth/build/gen"
	dp "deephealth/plugin"
	dt "deephealth/types"
)

var (
	report       = flag.Bool("report", true, "Whether to report events to health service")
	staleSeconds = flag.Int("stale", 5*60, "Cutoff in seconds to skip stale events. -1 means no check for staleness.")
	mergeSeconds = flag.Int("merge", 5, "Do not repeated report event for a subject within the given time.")
	log          = flag.String("log", "", "Log file to watch for (Required)")
	server       = flag.String("server", "", "Address of health server to report events to (Required)")
)

type report_key struct {
	subject string
	status  pb.Status
	score   int32
}

var lastReportTime = make(map[report_key]time.Time)
var ipEntities = make(map[string]string)
var staleCutoff float64
var mergeCutoff float64
var reportHandle uint64

func usage() {
	fmt.Printf("Usage: %s OPTIONS <plugin> [PLUGIN OPTIONS]...\n\n", os.Args[0])
	flag.PrintDefaults()
}

func reportEvent(client pb.HealthServiceClient, event *dt.Event) error {
	key := report_key{event.Subject, event.Status, int32(event.Score)}
	if mergeCutoff > 0 {
		ts, ok := lastReportTime[key]
		if ok && event.Time.Sub(ts).Seconds() < mergeCutoff {
			fmt.Printf("report for %s is too frequent, skip\n", event.Subject)
			return nil
		}
	}
	observation := dt.NewObservationSingleMetric(event.Time, event.Context, event.Status, event.Score)
	report := &pb.Report{
		Observer:    event.Id,
		Subject:     event.Subject,
		Observation: observation,
	}
	reply, err := client.SubmitReport(context.Background(), &pb.SubmitReportRequest{Handle: reportHandle, Report: report})
	if err != nil {
		return err
	}
	lastReportTime[key] = event.Time
	switch reply.Result {
	case pb.SubmitReportReply_ACCEPTED:
		fmt.Printf("Accepted report %s\n", event)
	case pb.SubmitReportReply_IGNORED:
		fmt.Printf("Ignored report %s\n", event)
	case pb.SubmitReportReply_FAILED:
		fmt.Printf("Failed report %s\n", event)
	}
	return nil
}

func main() {
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	if len(*log) == 0 {
		fmt.Println("Log file argument is empty")
		os.Exit(1)
	}
	var plugin dp.LogTailPlugin
	switch args[0] {
	case "zookeeper":
		plugin = new(dp.ZooKeeperPlugin)
	default:
		fmt.Println("Unsupported plugin " + args[0])
		os.Exit(1)
	}
	plugin.ProvideFlags().Parse(args[1:])
	err := plugin.ValidateFlags()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = plugin.Init()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	module := plugin.ProvideObserverModule()
	parser := plugin.ProvideEventParser()

	var client pb.HealthServiceClient
	if *report {
		addr := *server
		if len(*server) == 0 {
			host, err := os.Hostname()
			if err != nil {
				fmt.Printf("Fail to get host name. Use localhost instead")
				host = "localhost"
			}
			addr = host + ":6688"
		}
		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			panic(fmt.Sprintf("Could not connect to %s: %v", *server, err))
		}
		defer conn.Close()
		client = pb.NewHealthServiceClient(conn)

		reply, err := client.Register(context.Background(), &pb.RegisterRequest{Module: module.Module, Observer: module.Observer})
		if err != nil {
			panic(fmt.Sprintf("Fail to register with DeepHealth service: %v", err))
		}
		reportHandle = reply.Handle
	}

	fmt.Println("Sleeping 3 seconds to stabilize")
	time.Sleep(3 * time.Second)

	staleCutoff = float64(*staleSeconds)
	mergeCutoff = float64(*mergeSeconds)

	fmt.Println("Start monitoring " + *log)

	t, _ := tail.TailFile(*log, tail.Config{Follow: true})
	for line := range t.Lines {
		event := parser.ParseLine(line.Text)
		if event != nil {
			if staleCutoff > 0 && time.Since(event.Time).Seconds() > staleCutoff {
				fmt.Printf("Skip stale event: %s\n", event)
				continue
			}
			fmt.Println(event)
			if *report {
				err = reportEvent(client, event)
				if err != nil {
					fmt.Printf("Error in reporting event: %s\n", err)
				}
			}
		}
	}
}
