package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

var (
	nodeName string
	podReq   map[*v1.Pod]int
)

type predicateFunc func(node *v1.Node, pod *v1.Pod) bool
type priorityFunc func(node *v1.Node, pod *v1.Pod) int

type Scheduler struct {
	clientset  *kubernetes.Clientset
	podQueue   chan *v1.Pod
	nodeLister listersv1.NodeLister
	predicates []predicateFunc
	priorities []priorityFunc
}

func main() {
	fmt.Println("Hello!")

	rand.Seed(time.Now().Unix())

	podReq = make(map[*v1.Pod]int)

	podQueue := make(chan *v1.Pod, 300)
	defer close(podQueue)

	quit := make(chan struct{})
	defer close(quit)

	scheduler := NewScheduler(podQueue, quit)
	scheduler.Run(quit)
}

func NewScheduler(podQueue chan *v1.Pod, quit chan struct{}) Scheduler {
	buildClientSet()

	return Scheduler{
		clientset:  clientSet,
		podQueue:   podQueue,
		nodeLister: initInformers(clientSet, podQueue, quit),
		predicates: []predicateFunc{
			vGPUPredicate,
		},
		priorities: []priorityFunc{
			vGPUPriority,
		},
	}
}

func initInformers(clientset *kubernetes.Clientset, podQueue chan *v1.Pod, quit chan struct{}) listersv1.NodeLister {
	factory := informers.NewSharedInformerFactory(clientset, 0)

	nodeInformer := factory.Core().V1().Nodes()
	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node, ok := obj.(*v1.Node)
			if !ok {
				log.Println("this is not a node")
				return
			}
			log.Printf("New Node Added to Store: %s", node.GetName())
		},
	})

	podInformer := factory.Core().V1().Pods()
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod, ok := obj.(*v1.Pod)
			if !ok {
				log.Println("this is not a pod")
				return
			}
			if pod.Spec.NodeName == "" && pod.Spec.SchedulerName == schedulerName {
				podQueue <- pod
			}
		},
	})

	factory.Start(quit)
	return nodeInformer.Lister()
}

func (s *Scheduler) Run(quit chan struct{}) {
	wait.Until(s.ScheduleOne, 0, quit)
}

func (s *Scheduler) ScheduleOne() {
	pod := <-s.podQueue
	log.Println("found a pod to schedule:", pod.Namespace+"/"+pod.Name)

	// 找到最合适的Node
	node, err := s.findFit(pod)
	if err != nil {
		log.Println("cannot find node that fits pod", err.Error())
		return
	}

	// 绑定Node与Pod
	err = s.bindPod(pod, node)
	if err != nil {
		log.Println("failed to bind pod", err.Error())
		return
	}

	message := fmt.Sprintf("Placed pod [%s/%s] on %s\n", pod.Namespace, pod.Name, node)

	err = s.emitEvent(pod, message)
	if err != nil {
		log.Println("failed to emit scheduled event", err.Error())
		return
	}

	fmt.Println(message)
}

func (s *Scheduler) findFit(pod *v1.Pod) (string, error) {
	nodes, err := s.nodeLister.List(labels.Everything())
	if err != nil {
		return "", err
	}

	filteredNodes := s.runPredicates(nodes, pod)
	if len(filteredNodes) == 0 {
		return "", errors.New("failed to find node that fits pod")
	}
	priorities := s.runPriorities(filteredNodes, pod)
	return s.findBestNode(priorities), nil
}

func (s *Scheduler) findBestNode(priorities map[string]int) string {
	var maxP = -1
	var bestNode string
	for node, p := range priorities {
		// 找出剩余显存最小的Node
		if maxP == -1 || p < maxP {
			maxP = p
			bestNode = node
		}
	}
	return bestNode
}

func (s *Scheduler) runPredicates(nodes []*v1.Node, pod *v1.Pod) []*v1.Node {
	filteredNodes := make([]*v1.Node, 0)
	podReq[pod] = getPodGPUMemoryRequests(pod)
	log.Printf("podName: %s", pod.Name)
	for _, node := range nodes {
		if s.predicatesJudge(node, pod) {
			filteredNodes = append(filteredNodes, node)
		}
	}
	log.Println("nodes that fit:")
	for _, node := range filteredNodes {
		log.Println("  ", node.Name)
	}
	return filteredNodes
}

func (s *Scheduler) predicatesJudge(node *v1.Node, pod *v1.Pod) bool {
	for _, predicate := range s.predicates {
		if !predicate(node, pod) {
			return false
		}
	}
	return true
}

func vGPUPredicate(node *v1.Node, pod *v1.Pod) bool {
	if !isGPUResourcesNode(node) || isAssignedNode(node) {
		log.Printf("  node %s is't a gpu node, or it has been assigned.", node.Name)
		return false
	}
	return getNodeGPUMemoryResources(node) >= podReq[pod]
}

func (s *Scheduler) runPriorities(nodes []*v1.Node, pod *v1.Pod) map[string]int {
	priorities := make(map[string]int)
	for _, node := range nodes {
		for _, priority := range s.priorities {
			priorities[node.Name] += priority(node, pod)
		}
	}
	log.Println("calculated priorities:", priorities)
	return priorities
}

func vGPUPriority(node *v1.Node, pod *v1.Pod) int {
	return getNodeGPUMemoryResources(node) - podReq[pod]
}

func (s *Scheduler) bindPod(pod *v1.Pod, nodeName string) error {
	return s.clientset.CoreV1().Pods(pod.Namespace).Bind(&v1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		Target: v1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Node",
			Name:       nodeName,
		},
	})
}

func (s *Scheduler) emitEvent(pod *v1.Pod, message string) error {
	timestamp := time.Now().UTC()
	_, err := s.clientset.CoreV1().Events(pod.Namespace).Create(&v1.Event{
		Count:          1,
		Message:        message,
		Reason:         "Scheduled",
		LastTimestamp:  metav1.NewTime(timestamp),
		FirstTimestamp: metav1.NewTime(timestamp),
		Type:           "Normal",
		Source: v1.EventSource{
			Component: schedulerName,
		},
		InvolvedObject: v1.ObjectReference{
			Kind:      "Pod",
			Name:      pod.Name,
			Namespace: pod.Namespace,
			UID:       pod.UID,
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: pod.Name + "-",
		},
	})
	if err != nil {
		return err
	}
	return nil
}
