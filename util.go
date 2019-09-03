package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var clientSet *kubernetes.Clientset

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func outOfCluster() *rest.Config {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	return config
}

func buildClientSet() {
	var (
		config *rest.Config
		err    error
	)
	//先尝试以InCluster的方式获取，获取不到再以OutOfCluster的方式获取
	config, err = rest.InClusterConfig()
	if err != nil {
		config = outOfCluster()
	}
	clientSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}
}

// true: 已分配
func isAssignedNode(node *v1.Node) bool {
	// 找到 node 中所有正在运行的 Pod
	selector := fields.SelectorFromSet(fields.Set{"spec.nodeName": node.Name, "status.phase": "Running"})
	podList, err := clientSet.CoreV1().Pods(v1.NamespaceAll).List(metav1.ListOptions{
		FieldSelector: selector.String(),
	})
	if err != nil {
		log.Printf("failed to get Pods assigned to node %v", nodeName)
		return false
	}

	for _, pod := range podList.Items {
		for _, container := range pod.Spec.Containers {
			limits := container.Resources.Limits[resourceName]
			if limits.Value() != 0 {
				return true
			}
		}
	}

	return false
}

func isGPUResourcesNode(node *v1.Node) bool {
	limits := getNodeGPUMemoryResources(node)
	log.Printf("  nodeName: %s, limits: %d", node.Name, limits)
	return limits > 0
}

func getNodeGPUMemoryResources(node *v1.Node) int {
	capacity := node.Status.Capacity[resourceName]
	return int(capacity.Value())
}

func getPodGPUMemoryRequests(pod *v1.Pod) int {
	var resourceRequest int64
	for _, container := range pod.Spec.Containers {
		limits := container.Resources.Limits[resourceName]
		resourceRequest += limits.Value()
	}
	return int(resourceRequest)
}
