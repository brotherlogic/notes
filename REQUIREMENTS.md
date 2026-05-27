# Notes Management System - Kubernetes Infrastructure Requirements

This document outlines the cluster infrastructure and environment configurations required to host and run the **Notes Management System** production server. Use these specifications to file infrastructure tickets against the Kubernetes cluster.

---

## 1. Persistent Storage (Volume Claim) Requirements

To prevent ephemeral container storage from wiping downloaded notebook page images upon container restarts, we require a persistent shared volume.

### Specification
* **Volume Access Mode**: `ReadWriteMany` (RWX). This allows scaling `notes-server` pods across multiple physical nodes.
* **StorageClass**: `rook-cephfs` (Rook/Ceph File System provider).
* **Storage Capacity**: `10Gi` (Gigabytes) requested.
* **Pod Mount Path**: Mount to the container at `/data/binaries`.

### Target PVC Manifest (`pvc.yaml`)
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: notes-binaries-pvc
  namespace: notes
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: rook-cephfs
  resources:
    requests:
      storage: 10Gi
```

---

## 2. Environment Variables & Secret Configuration

The backend container expects the following environment variables injected during runtime. Sensitive credentials (OAuth client IDs/secrets) must be mapped from Kubernetes Secrets.

| Environment Variable | Description | Required | Default / Example Value |
| :--- | :--- | :--- | :--- |
| `PORT` | The port on which the HTTP server listens inside the container. | No | `8080` |
| `DATA_DIR` | Filesystem path pointing to the mounted persistent volume. | No | `/data/binaries` |
| `FRONTEND_DIR` | Filesystem directory containing compiled React static assets. | No | `./frontend/dist` |
| `GITHUB_CLIENT_ID` | OAuth application Client ID generated on GitHub. | **Yes** | *[Retrieve from Secret]* |
| `GITHUB_CLIENT_SECRET` | OAuth application Client Secret generated on GitHub. | **Yes** | *[Retrieve from Secret]* |
| `GDRIVE_CLIENT_ID` | OAuth application Client ID generated on Google Cloud Console. | **Yes** | *[Retrieve from Secret]* |
| `GDRIVE_CLIENT_SECRET`| OAuth application Client Secret generated on Google Cloud Console. | **Yes** | *[Retrieve from Secret]* |
| `PSTORE_ADDRESS` | Address location for the `brotherlogic/pstore` gRPC database service. | **Yes** | `pstore.database.svc.cluster.local:50051` |

---

## 3. Deployment Configuration Snippet (`deployment.yaml`)

Use the following manifest snippet when building deployment specs for the app:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: notes-server
  namespace: notes
spec:
  replicas: 3
  selector:
    matchLabels:
      app: notes-server
  template:
    metadata:
      labels:
        app: notes-server
    spec:
      containers:
        - name: notes-server
          image: brotherlogic/notes:latest
          ports:
            - containerPort: 8080
          env:
            - name: PORT
              value: "8080"
            - name: DATA_DIR
              value: /data/binaries
            - name: FRONTEND_DIR
              value: /app/frontend/dist
            - name: PSTORE_ADDRESS
              value: pstore.database.svc.cluster.local:50051
            - name: GITHUB_CLIENT_ID
              valueFrom:
                secretKeyRef:
                  name: oauth-secrets
                  key: github-client-id
            - name: GITHUB_CLIENT_SECRET
              valueFrom:
                secretKeyRef:
                  name: oauth-secrets
                  key: github-client-secret
            - name: GDRIVE_CLIENT_ID
              valueFrom:
                secretKeyRef:
                  name: oauth-secrets
                  key: google-client-id
            - name: GDRIVE_CLIENT_SECRET
              valueFrom:
                secretKeyRef:
                  name: oauth-secrets
                  key: google-client-secret
          volumeMounts:
            - name: binary-storage
              mountPath: /data/binaries
      volumes:
        - name: binary-storage
          persistentVolumeClaim:
            claimName: notes-binaries-pvc
```

---

## 4. Networking & Service Routing

To route traffic securely to the application pods, we require a Kubernetes Service and an Ingress controller mapping.

### Target Service Manifest (`service.yaml`)
```yaml
apiVersion: v1
kind: Service
metadata:
  name: notes-server-svc
  namespace: notes
spec:
  selector:
    app: notes-server
  ports:
    - protocol: TCP
      port: 8080
      targetPort: 8080
  type: ClusterIP
```

### Target Ingress Manifest (`ingress.yaml`)
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: notes-ingress
  namespace: notes
  annotations:
    kubernetes.io/ingress.class: nginx
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  rules:
    - host: notes.brotherlogic.org
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: notes-server-svc
                port:
                  number: 8080
```

---

## 5. Deployment Instructions for brotherlogic/prod

To release these infrastructure templates into the production environment via your cluster repository (`brotherlogic/prod`):

1. **Directories**: Copy the YAML templates (`pvc.yaml`, `deployment.yaml`, `service.yaml`, and `ingress.yaml`) defined in this specifications file into the appropriate app directory (e.g. `apps/notes-management/`) in the `brotherlogic/prod` repository.
2. **Secrets Setup**: Ensure that the `oauth-secrets` Secret exists in the target `notes` namespace of your cluster containing base64-encoded credentials for the following keys before deploying:
   * `github-client-id`
   * `github-client-secret`
   * `google-client-id`
   * `google-client-secret`
3. **Application**: Apply the manifest tree using your GitOps workflow or execute `kubectl apply -f .` within the directory to roll out the persistent volumes, cluster routing services, and load balanced server deployment.

