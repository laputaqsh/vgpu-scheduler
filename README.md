# VMware vGPU Scheduler for Kubernetes

## About

The VMware vGPU Scheduler for Kubernetes is a Scheduler Extender that allows you to deploy the vGPU Pod to most suitable Node. See more about [Kubernetes Scheduler](https://kubernetes.io/docs/concepts/scheduling/kube-scheduler/). 

## Prerequisites

The list of prerequisites for running the Vmware vGPU Scheduler is described below:
* Kubernetes version >= 1.15
* Golang version >= 1.12

## Quick Start

### In Kubernetes Cluster

Scheduler needs to interact with Kubernetes resources like any other API resource(via kubectl, API calls, etc.). So we need to gain the [access authority](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#role-and-clusterrole) of Kubernetes as following:
```shell
$ kubectl create -f https://raw.githubusercontent.com/laputaq/vgpu-scheduler/master/deployment/rbac.yaml
```
After setting up the serviceAccount. We can deploy the vGPU Scheduler as Deployment as following:
```shell
$ kubectl create -f https://raw.githubusercontent.com/laputaq/vgpu-scheduler/master/deployment/deployment.yaml
```
> *if `hostNetwork` is not set as `true`, it will have an issue with Scheduler not being able to access the Kubernetes Network, which appears to be due to the empty KUBE-MARK-MASQ forwarded by KUBE-SERVICES in iptables.*

### Out of Kubernetes Cluster

#### With Docker

##### Build
```shell
$ docker pull laputaq/vgpu-scheduler:v1.0
```

##### Run
```shell
$ docker run -d --name vgpu-scheduler laputaq/vgpu-scheduler:v1.0
```

#### Without Docker

##### Build
```shell
$ git clone https://github.com/laputaq/vgpu-scheduler.git && cd vgpu-scheduler
$ go build
```

##### Run
```shell
$ ./vgpu-scheduler
```