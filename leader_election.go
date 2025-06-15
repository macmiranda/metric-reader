package main

import (
    "context"
    "os"
    "sync/atomic"
    "time"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/leaderelection"
    "k8s.io/client-go/tools/leaderelection/resourcelock"

    "github.com/rs/zerolog/log"
)

// leaderActive is set to true only in the pod currently holding leadership.
var leaderActive atomic.Bool

// IsLeader reports whether this instance currently owns the leader lock.
func IsLeader() bool {
    return leaderActive.Load()
}

// startLeaderElection initialises the optional Kubernetes leader-election process.
// When leader-election is disabled (LEADER_ELECTION_ENABLED=false) the function
// simply marks the instance as leader and returns.
func startLeaderElection(ctx context.Context) {
    // Leader-election can be opted-out via env var.
    if v := os.Getenv("LEADER_ELECTION_ENABLED"); v == "false" {
        leaderActive.Store(true)
        log.Info().Msg("leader election disabled via LEADER_ELECTION_ENABLED, executing actions on every replica")
        return
    }

    hostname, _ := os.Hostname()

    lockName := os.Getenv("LEADER_ELECTION_LOCK_NAME")
    if lockName == "" {
        lockName = "metric-reader-leader"
    }

    namespace := os.Getenv("POD_NAMESPACE")
    if namespace == "" {
        // When running in-cluster the POD_NAMESPACE env var can be injected via the
        // Downward API. Fallback to `default` if it's not present so we still work
        // outside of a cluster (e.g. for tests).
        namespace = "default"
    }

    cfg, err := rest.InClusterConfig()
    if err != nil {
        // If we cannot obtain an in-cluster config (e.g. when running locally)
        // assume single-replica and skip leader-election.
        leaderActive.Store(true)
        log.Warn().Err(err).Msg("unable to get in-cluster config, skipping leader election")
        return
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