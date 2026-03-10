# ZenContext in-cluster (Block 4)

Deploys Redis and MinIO inside the cluster so zen-brain components (e.g. Foreman, zen-brain) can use ZenContext without external dependencies.

## Deploy

```bash
kubectl apply -f namespace.yaml
kubectl apply -f redis.yaml
kubectl apply -f minio.yaml
```

Or from repo root:

```bash
kubectl apply -f deployments/zencontext-in-cluster/
```

## Connect from inside the cluster

- **Redis (Tier 1):** Use service DNS: `redis://zencontext-redis.zen-context.svc.cluster.local:6379`  
  - Env: `REDIS_URL=redis://zencontext-redis.zen-context.svc.cluster.local:6379`  
  - Or from the same namespace: `REDIS_URL=redis://zencontext-redis:6379`

- **MinIO (Tier 3 / S3):** Use service DNS: `http://zencontext-minio.zen-context.svc.cluster.local:9000`  
  - ZenContext S3 config: set endpoint to that URL, bucket e.g. `zen-brain-context`, use path-style, region optional.  
  - Create the bucket once (e.g. via MinIO console or `mc mb`).

## Env for zen-brain / ZenContext factory

When running in the same cluster (e.g. Foreman or zen-brain in Kubernetes):

- `REDIS_URL=redis://zencontext-redis.zen-context.svc.cluster.local:6379`
- Tier 3 (S3): endpoint `http://zencontext-minio.zen-context.svc.cluster.local:9000`, bucket `zen-brain-context`, `UsePathStyle: true`, access/secret from MinIO (default minioadmin/minioadmin for dev only).

## Create MinIO bucket (one-time)

From a pod with `mc` or port-forward MinIO and use AWS CLI / mc:

```bash
kubectl run -it --rm mc --image=minio/mc --restart=Never -- \
  sh -c 'mc alias set myminio http://zencontext-minio.zen-context:9000 minioadmin minioadmin && mc mb myminio/zen-brain-context --ignore-existing'
```

(Adjust namespace if not `zen-context`.)
