# nfs-client-provisioner

This repository is based on an extract from [kubernetes-incubator/external-storage](https://github.com/kubernetes-incubator/external-storage/tree/e35bdf47/nfs-client) since I needed it when the upstream got archived/discontinued and couldn't find a new upstream. Since the creation of this repository, the code base has diverged a bit to allow for volumes shared between namespaces (throught the use of "conflicting" nfs path).

# Distribution
## Docker
Docker images of this projet are available for arm/v7, arm64/v8 and amd64 at [vbouchaud/nfs-client-provisioner](https://hub.docker.com/r/vbouchaud/nfs-client-provisioner) on docker hub and on quay.io at [vbouchaud/nfs-client-provisioner](https://quay.io/vbouchaud/nfs-client-provisioner).

## Binary
Binaries for the following OS and architectures are available on the release page:
 - linux/arm64
 - linux/arm
 - linux/amd64

## Kubernetes
### Helm Chart

Though the project forked, you can still follow the instructions for the stable helm chart maintained at https://github.com/helm/charts/tree/master/stable/nfs-client-provisioner

The tl;dr is

```bash
$ helm install stable/nfs-client-provisioner --set nfs.server=x.x.x.x --set nfs.path=/exported/path
```

### Without Helm
Get all of the files in the [deploy](https://github.com/kubernetes-incubator/external-storage/tree/master/nfs-client/deploy) directory of this repository.

You must edit the provisioner's deployment file to add connection information for your NFS server. Edit `deploy/deployment.yaml` and replace the two occurences of <YOUR NFS SERVER HOSTNAME> with your server's hostname.

```yaml
kind: Deployment
apiVersion: apps/v1
metadata:
  name: nfs-client-provisioner
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nfs-client-provisioner
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: nfs-client-provisioner
    spec:
      serviceAccountName: nfs-client-provisioner
      containers:
        - name: nfs-client-provisioner
          image: quay.io/vbouchaud/nfs-client-provisioner:latest
          volumeMounts:
            - name: nfs-client-root
              mountPath: /persistentvolumes
          env:
            - name: PROVISIONER_NAME
              value: fuseim.pri/ifs
            - name: NFS_SERVER
              value: <YOUR NFS SERVER HOSTNAME>
            - name: NFS_PATH
              value: /var/nfs
      volumes:
        - name: nfs-client-root
          nfs:
            server: <YOUR NFS SERVER HOSTNAME>
            path: /var/nfs
```

You may also want to change the PROVISIONER_NAME above from ``fuseim.pri/ifs`` to something more descriptive like ``nfs-storage``, but if you do remember to also change the PROVISIONER_NAME in the storage class definition below:

This is `deploy/class.yaml` which defines the NFS-Client's Kubernetes Storage Class:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: managed-nfs-storage
provisioner: fuseim.pri/ifs # or choose another name, must match deployment's env PROVISIONER_NAME'
parameters:
  archiveOnDelete: "false" # When set to "false" your PVs will not be archived
                           # by the provisioner upon deletion of the PVC.
```

**Step 5: Finally, test your environment!**

Now we'll test your NFS provisioner.

Deploy:

```sh
$ kubectl create -f deploy/test-claim.yaml -f deploy/test-pod.yaml
```

Now check your NFS Server for the file `SUCCESS`.

```sh
kubectl delete -f deploy/test-pod.yaml -f deploy/test-claim.yaml
```

Now check the folder has been deleted.

**Step 6: Deploying your own PersistentVolumeClaims**. To deploy your own PVC, make sure that you have the correct `storage-class` as indicated by your `deploy/class.yaml` file.

For example:

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: test-claim
  annotations:
    volume.beta.kubernetes.io/storage-class: "managed-nfs-storage"
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 1Mi
```
