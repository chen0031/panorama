package store

import (
	"fmt"
	"sync"
	"testing"
	"time"

	pb "deephealth/build/gen"

	dh "deephealth"
	dt "deephealth/types"
)

func TestAddSubject(t *testing.T) {
	dh.SetLogLevel(dh.InfoLevel)
	store := NewRawHealthStorage("TS_1", "TS_2")
	metrics := map[string]*pb.Value{"cpu": &pb.Value{pb.Status_HEALTHY, 100}}
	report := dt.NewReport("FE_2", "TS_3", metrics)
	result, err := store.AddReport(report, true)
	if err != nil {
		t.Errorf("Fail to add report %s", report)
	}
	if result != REPORT_IGNORED {
		t.Errorf("Report %s should get ignored", report)
	}
	store.AddSubject("TS_3")
	result, err = store.AddReport(report, true)
	if err != nil {
		t.Errorf("Fail to add report %s", report)
	}
	if result != REPORT_ACCEPTED {
		t.Errorf("Report %s should get accepted", report)
	}
}

func TestAddReport(t *testing.T) {
	dh.SetLogLevel(dh.InfoLevel)
	subjects := []string{"TS_1", "TS_2", "TS_3", "TS_4"}
	smap := make(map[string]bool)
	for _, s := range subjects {
		smap[s] = true
	}

	store := NewRawHealthStorage(subjects...)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		t.Logf("Making observation %d", i)
		metrics := map[string]*pb.Value{
			"cpu":     &pb.Value{pb.Status_HEALTHY, 100},
			"disk":    &pb.Value{pb.Status_HEALTHY, 90},
			"network": &pb.Value{pb.Status_UNHEALTHY, 10},
			"memory":  &pb.Value{pb.Status_MAYBE_UNHEALTHY, 30},
		}
		observer := fmt.Sprintf("FE_%d", i)
		subject := fmt.Sprintf("TS_%d", i%3)
		report := dt.NewReport(observer, subject, metrics)
		wg.Add(1)
		go func() {
			result, err := store.AddReport(report, true)
			if err != nil {
				t.Errorf("Fail to add report %s", report)
			}
			_, watched := smap[subject]
			if watched && result == REPORT_IGNORED {
				t.Errorf("Report %s should not get ignored", report)
			}
			wg.Done()
		}()
	}
	wg.Wait()

	if len(store.Tenants) == 0 {
		t.Error("Health table should not be empty")
	}
	for subject, stereo := range store.Tenants {
		t.Logf("=============%s=============", subject)
		for observer, view := range stereo.Views {
			t.Logf("%d observations for %s->%s", len(view.Observations), observer, subject)
			for _, ob := range view.Observations {
				t.Logf("|%s| %s\n", observer, dt.ObservationString(ob))
			}
		}
	}
}

func TestRecentReport(t *testing.T) {
	dh.SetLogLevel(dh.InfoLevel)
	store := NewRawHealthStorage("TS_1", "TS_2")

	metrics := map[string]*pb.Value{"cpu": &pb.Value{pb.Status_HEALTHY, 100}}
	report := dt.NewReport("FE_2", "TS_1", metrics)
	store.AddReport(report, true)
	metrics = map[string]*pb.Value{"cpu": &pb.Value{pb.Status_HEALTHY, 90}}
	report = dt.NewReport("FE_2", "TS_1", metrics)
	store.AddReport(report, true)
	metrics = map[string]*pb.Value{"cpu": &pb.Value{pb.Status_HEALTHY, 70}}
	report = dt.NewReport("FE_2", "TS_1", metrics)
	store.AddReport(report, true)
	metrics = map[string]*pb.Value{"cpu": &pb.Value{pb.Status_UNHEALTHY, 30}}
	report = dt.NewReport("FE_2", "TS_1", metrics)
	store.AddReport(report, true)

	ret := store.GetLatestReport("TS_1")
	if ret.Observer != "FE_2" {
		t.Errorf("Wrong subject in the latest report: %s\n", *ret)
	}
	metric, ok := ret.Observation.Metrics["cpu"]
	if !ok {
		t.Error("The latest report have a CPU metric")
	}
	if metric.Value.Status != pb.Status_UNHEALTHY || metric.Value.Score != 30 {
		t.Errorf("Wrong metric in the latest report: %s\n", metric)
	}

	time.Sleep(200 * time.Millisecond)
	metrics = map[string]*pb.Value{"memory": &pb.Value{pb.Status_UNHEALTHY, 20}}
	report = dt.NewReport("FE_4", "TS_1", metrics)
	store.AddReport(report, true)
	ret = store.GetLatestReport("TS_1")
	if ret.Observer != "FE_4" {
		t.Errorf("Wrong subject in the latest report: %s\n", *ret)
	}
	metric, ok = ret.Observation.Metrics["memory"]
	if !ok {
		t.Error("The latest report have a memory metric")
	}
	if metric.Value.Status != pb.Status_UNHEALTHY || metric.Value.Score != 20 {
		t.Errorf("Wrong metric in the latest report: %s\n", metric)
	}

	time.Sleep(200 * time.Millisecond)
	metrics = map[string]*pb.Value{"network": &pb.Value{pb.Status_HEALTHY, 80}}
	report = dt.NewReport("FE_5", "TS_1", metrics)
	store.AddReport(report, true)
	metrics = map[string]*pb.Value{"memory": &pb.Value{pb.Status_HEALTHY, 70}}
	report = dt.NewReport("FE_1", "TS_1", metrics)
	store.AddReport(report, true)
	ret = store.GetLatestReport("TS_1")
	if ret.Observer != "FE_1" {
		t.Errorf("Wrong subject in the latest report: %s\n", *ret)
	}
	metric, ok = ret.Observation.Metrics["memory"]
	if !ok {
		t.Error("The latest report have a memory metric")
	}
	if metric.Value.Status != pb.Status_HEALTHY || metric.Value.Score != 70 {
		t.Errorf("Wrong metric in the latest report: %s\n", metric)
	}
}