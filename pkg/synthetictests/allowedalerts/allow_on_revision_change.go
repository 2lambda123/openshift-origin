package allowedalerts

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"github.com/openshift/origin/pkg/synthetictests/platformidentification"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type etcdRevisionChangeAllowance struct {
	operatorClient   operatorv1client.OperatorV1Interface
	startingRevision int
}

var allowedWhenEtcdRevisionChange = &etcdRevisionChangeAllowance{}
var _ AlertTestAllowanceCalculator = &etcdRevisionChangeAllowance{}

func NewAllowedWhenEtcdRevisionChange(ctx context.Context, operatorClient operatorv1client.OperatorV1Interface) (*etcdRevisionChangeAllowance, error) {
	biggestRevision, err := GetBiggestRevisionForEtcdOperator(ctx, operatorClient)
	if err != nil {
		return nil, err
	}
	return &etcdRevisionChangeAllowance{
		operatorClient:   operatorClient,
		startingRevision: biggestRevision,
	}, nil
}

func (d *etcdRevisionChangeAllowance) FailAfter(alertName string, jobType platformidentification.JobType) (time.Duration, error) {
	biggestRevision, err := GetBiggestRevisionForEtcdOperator(context.TODO(), d.operatorClient)
	if err != nil {
		return 0, err
	}
	// if the number of revisions is different compared to what we have collected at the beginning of the test suite
	// increase allowed time for the alert
	// the rationale is that some tests might roll out a new version of etcd during each rollout we allow max 3 elections per revision (we assume there are 3 master machines at most)
	// in the future, we could make this function more dynamic
	// we will leave it simple for now
	numberOfRevisions := biggestRevision - d.startingRevision
	if numberOfRevisions > 2 {
		return time.Duration(numberOfRevisions) * 10 * time.Minute, nil

	}
	allowed, _, _ := getClosestPercentilesValues(alertName, jobType)
	return allowed.P99, nil
}

func (d *etcdRevisionChangeAllowance) FlakeAfter(alertName string, jobType platformidentification.JobType) time.Duration {
	allowed, _, _ := getClosestPercentilesValues(alertName, jobType)
	return allowed.P95
}

// GetBiggestRevisionForEtcdOperator calculates the biggest revision among replicas of the most recently successful deployment
func GetBiggestRevisionForEtcdOperator(ctx context.Context, operatorClient operatorv1client.OperatorV1Interface) (int, error) {
	etcd, err := operatorClient.Etcds().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return 0, err
	}
	biggestRevision := 0
	for _, nodeStatus := range etcd.Status.NodeStatuses {
		if int(nodeStatus.CurrentRevision) > biggestRevision {
			biggestRevision = int(nodeStatus.CurrentRevision)
		}
	}
	return biggestRevision, nil
}

// GetEstimatedNumberOfRevisionsForEtcdOperator calculates the number of revisions that have occurred between now and duration
func GetEstimatedNumberOfRevisionsForEtcdOperator(ctx context.Context, kubeClient kubernetes.Interface, duration time.Duration) (int, error) {
	configMaps, err := kubeClient.CoreV1().ConfigMaps("openshift-etcd").List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	var revisionCounter int
	allowedRevisionsTS := time.Now().Add(-duration)
	for _, configMap := range configMaps.Items {
		if strings.Contains(configMap.Name, "revision-status-") {
			sub := strings.Split(configMap.Name, "-")
			if len(sub) != 3 {
				return 0, fmt.Errorf("the configmap: %v has an incorrect name, unable to extract the revision number", configMap.Name)
			}
			_, err := strconv.Atoi(sub[2])
			if err != nil {
				return 0, err
			}
			if configMap.CreationTimestamp.After(allowedRevisionsTS) {
				revisionCounter++
			}
		}
	}
	return revisionCounter, nil
}
