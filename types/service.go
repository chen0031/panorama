package types

import (
	"sync"
	"time"
)

type HealthStorage interface {
	// Add a subject to the observing subject list
	AddSubject(subject EntityId) bool

	// Stop observing a particular subject, all the reports
	// concerning this subject will be ignored
	RemoveSubject(subject EntityId, clean bool) bool

	// Add a report to the view storage
	AddReport(report *Report, filter bool) (int, error)

	// Get the latest report for a subject
	GetLatestReport(subject EntityId) *Report

	// Get the whole panorama for a subject
	GetPanorama(subject EntityId) (*Panorama, *sync.Mutex)

	// Get the view from an observer about a subject
	GetView(observer EntityId, subject EntityId) (*View, *sync.Mutex)
}

type HealthInference interface {
	// Infer the health of a subject given a new report
	Infer(report *Report) (*Inference, error)

	// Get the health inference of a subject
	GetInference(subject EntityId) *Inference

	// Start the inference service
	Start() error

	// Stop the inference service
	Stop() error
}

type PingReply struct {
	Ts time.Time
}

type HealthExchange interface {
	// Propagate a report to other peers
	Propagate(report *Report) error

	// Ping one peer and get a response
	Ping(peer EntityId) (*PingReply, error)

	// Ping all peers and get response
	PingAll() (map[EntityId]*PingReply, error)

	// peer is interested in a particular subject
	Interested(peer EntityId, subject EntityId) bool

	// peer is not interested in a particular subject
	Uninterested(peer EntityId, subject EntityId) bool
}
