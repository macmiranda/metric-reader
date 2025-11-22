package main

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

// leaderActive is set to true only in the pod currently holding leadership.
var leaderActive atomic.Bool

// IsLeader reports whether this instance currently owns the leader lock.
func IsLeader() bool {
	return leaderActive.Load()
}

// startLeaderElection initialises the optional Kubernetes leader-election process.
// When leader-election is disabled the function simply marks the instance as leader and returns.
func startLeaderElection(ctx context.Context, config *Config) {
	zerologAdapter := zerologr.New(&log.Logger)
	klog.SetLogger(zerologAdapter)

	// Leader-election can be opted-out via config.
	if !config.LeaderElectionEnabled {
		leaderActive.Store(true)
		log.Info().Msg("leader election disabled, executing actions on every replica")
		return
	}

	hostname, _ := os.Hostname()

	lockName := config.LeaderElectionLockName

	namespace := config.LockNamespace

	cfg, err := rest.InClusterConfig()
	if err != nil {
		// If we cannot obtain an in-cluster config (e.g. when running locally)
		// assume single-replica and skip leader-election.
		leaderActive.Store(true)
		log.Warn().Err(err).Msg("unable to get in-cluster config, skipping leader election")
		return
	}

	// If namespace is not set, try to detect it from the service account
	if namespace == "" {
		namespaceBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			leaderActive.Store(true)
			log.Warn().Err(err).Msg("unable to detect namespace from service account, skipping leader election")
			return
		}
		namespace = strings.TrimSpace(string(namespaceBytes))
		log.Info().Str("namespace", namespace).Msg("auto-detected namespace from service account")
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		leaderActive.Store(true)
		log.Warn().Err(err).Msg("unable to build kubernetes client, skipping leader election")
		return
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      lockName,
			Namespace: namespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: hostname,
		},
	}

	// Leader-election life-cycle callbacks.
	lec := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(c context.Context) {
				leaderActive.Store(true)
				log.Info().Msg("gained leadership; actions will be executed from this replica")
			},
			OnStoppedLeading: func() {
				leaderActive.Store(false)
				log.Warn().Msg("lost leadership; terminating to allow another instance to take over")
				os.Exit(1)
			},
			OnNewLeader: func(id string) {
				if id != hostname {
					leaderActive.Store(false)
				}
				log.Info().Str("leader", id).Msg("current metric-reader leader")
			},
		},
	}

	// Run leader-election in a background goroutine so main can continue.
	go leaderelection.RunOrDie(ctx, lec)
}
