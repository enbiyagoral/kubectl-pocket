# kubectl-pocket

A handy kubectl plugin for common database operations in Kubernetes.

## Install

```bash
go build -o kubectl-pocket .
mv kubectl-pocket /usr/local/bin/
```

## Usage

### Test database connections

```bash
# MongoDB
kubectl pocket test mongo mongodb://mongo-svc:27017

# PostgreSQL
kubectl pocket test postgres postgres://pg-svc:5432/mydb

# Redis
kubectl pocket test redis redis-svc:6379
```

### Open database shell

```bash
kubectl pocket test mongo mongodb://mongo-svc:27017 --shell
kubectl pocket test postgres postgres://pg-svc:5432/mydb --shell
kubectl pocket test redis redis-svc:6379 --shell
```

### Port-forward

```bash
kubectl pocket pf redis           # localhost:6379
kubectl pocket pf mongo           # localhost:27017
kubectl pocket pf postgres        # localhost:5432
kubectl pocket pf redis 16379     # custom local port
```

### Flags

```bash
-n, --namespace string   # target namespace
--kubeconfig string      # kubeconfig path
--timeout duration       # connection timeout (default 30s)
```

## How it works

- Creates a temporary pod with the database client
- Runs connection test or opens interactive shell
- Cleans up the pod automatically on exit
