# How to use it
## In Kubernetes Cluster
``` bash
curl -O https://raw.githubusercontent.com/laputaq/vgpu-scheduler/master/deployment/rbac.yaml
kubectl apply -f rbac.yaml

curl -O https://raw.githubusercontent.com/laputaq/vgpu-scheduler/master/deployment/deployment.yaml
kubectl apply -f deployment.yaml
```

## Out of Kubernetes Cluster
### With Docker
#### Build
``` bash
docker pull laputaq/vgpu-scheduler:v1.0
```

#### Run
``` bash
docker run -d --name vgpu-scheduler laputaq/vgpu-scheduler:v1.0
```

### Without Docker
#### Build
``` bash
git clone https://github.com/laputaq/vgpu-scheduler.git && cd vgpu-scheduler
go build
```

#### Run
``` bash
./vgpu-scheduler
```
