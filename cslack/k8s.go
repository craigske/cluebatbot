package cslack

import (
	"context"
	"fmt"

	"github.com/golang/glog"
	"golang.org/x/oauth2/google"
	container "google.golang.org/api/container/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ConnectToGoogleCloudAPI connects...
func ConnectToGoogleCloudAPI() bool {
	// google connect
	ctx := context.Background()
	hc, err := google.DefaultClient(ctx, container.CloudPlatformScope)
	if err != nil {
		glog.Errorf("Could not get authenticated client: %v", err)
	} else {
		svc, err := container.New(hc)
		if err != nil {
			glog.Errorf("Could not initialize gke client: %v", err)
		} else {
			if err := listClusters(svc, "craigskelton-com", "us-central1-a"); err != nil {
				glog.Errorf("list clusters error: %s", err)
			}
		}
	}
	return false
}

// GetK8sPodInfo creates the in-cluster config
func GetK8sPodInfo() {
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Errorln(err.Error())
	} else {
		// creates the clientset
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			glog.Errorln(err.Error())
		} else {
			pods, err := clientset.CoreV1().Pods("cluebatbot").List(metav1.ListOptions{})
			if err != nil {
				glog.Errorln(err.Error())
			}
			glog.Infof("There are %d pods in the cluster\n", len(pods.Items))
			// Examples for error handling:
			// - Use helper functions like e.g. errors.IsNotFound()
			// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
			_, err = clientset.CoreV1().Pods("cluebatbot").Get("*", metav1.GetOptions{})
			if errors.IsNotFound(err) {
				glog.Infof("Pod not found\n")
			} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
				glog.Infof("Error getting pod %v\n", statusError.ErrStatus.Message)
			} else if err != nil {
				glog.Errorf(err.Error())
			} else {
				glog.Errorf("Found pod\n")
			}
		}
	}
}

func listClusters(svc *container.Service, projectID, zone string) error {
	list, err := svc.Projects.Zones.Clusters.List(projectID, zone).Do()
	if err != nil {
		return fmt.Errorf("failed to list clusters: %v", err)
	}
	for _, v := range list.Clusters {
		glog.Infof("Cluster %q (%s) master_version: v%s", v.Name, v.Status, v.CurrentMasterVersion)

		poolList, err := svc.Projects.Zones.Clusters.NodePools.List(projectID, zone, v.Name).Do()
		if err != nil {
			return fmt.Errorf("failed to list node pools for cluster %q: %v", v.Name, err)
		}
		for _, np := range poolList.NodePools {
			glog.Infof("  -> Pool %q (%s) machineType=%s node_version=v%s autoscaling=%v", np.Name, np.Status,
				np.Config.MachineType, np.Version, np.Autoscaling != nil && np.Autoscaling.Enabled)
		}
	}
	return nil
}
