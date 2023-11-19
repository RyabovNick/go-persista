# Go-persista

## How to run in k8s

1. Build image: `docker build -t go-persista .`
2. Create PVC: `kubectl apply -f .\.deploy\pvc.yaml`
3. Create deployment: `kubectl apply -f .\.deploy\deployment.yaml`
4. Get Helm repository info:
    - `helm repo add prometheus-community https://prometheus-community.github.io/helm-charts`
    - `helm repo update`
5. And install prometheus: `helm install prometheus prometheus-community/kube-prometheus-stack`
6. Create Service Monitor: `kubectl apply -f .\.deploy\servicemonitor.yaml`
