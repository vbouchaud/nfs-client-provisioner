/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	core "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"
)

const (
	provisionerNameKey  = "PROVISIONER_NAME"
	sharedAnnotationKey = "nfs-provisioner.legion.bouchaud.org/shared-with-key"
)

type nfsProvisioner struct {
	client kubernetes.Interface
	server string
	path   string
}

const (
	mountPath = "/persistentvolumes"
)

var _ controller.Provisioner = &nfsProvisioner{}

func (p *nfsProvisioner) Provision(_ context.Context, options controller.ProvisionOptions) (*core.PersistentVolume, controller.ProvisioningState, error) {
	if options.PVC.Spec.Selector != nil {
		return nil, controller.ProvisioningFinished, fmt.Errorf("claim Selector is not supported")
	}
	log.Info().Msgf("nfs provisioner: VolumeOptions %v", options)

	var dirName string
	var namespace string
	var shared = false

	namespace = options.PVC.Namespace

	if _, ok := options.PVC.Annotations[sharedAnnotationKey]; ok {
		shared = true
	}

	if shared {
		dirName = strings.Join([]string{"shared", options.PVC.Annotations[sharedAnnotationKey]}, "-")
	} else {
		dirName = strings.Join([]string{namespace, options.PVC.Name, options.PVName}, "-")
	}

	if *options.StorageClass.ReclaimPolicy != core.PersistentVolumeReclaimRetain && shared {
		return nil, controller.ProvisioningFinished, errors.New("not allowed to create shared volatile pv.")
	}

	fullPath := filepath.Join(mountPath, dirName)
	log.Info().Msgf("creating path %s", fullPath)

	if err := os.MkdirAll(fullPath, 0777); err != nil {
		return nil, controller.ProvisioningFinished, errors.New("unable to create directory to provision new pv: " + err.Error())
	}

	// if path is already a directory, still chown
	os.Chmod(fullPath, 0777)

	pv := &core.PersistentVolume{
		ObjectMeta: meta.ObjectMeta{
			Name: options.PVName,
		},
		Spec: core.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: *options.StorageClass.ReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			MountOptions:                  options.StorageClass.MountOptions,
			Capacity: core.ResourceList{
				core.ResourceName(core.ResourceStorage): options.PVC.Spec.Resources.Requests[core.ResourceName(core.ResourceStorage)],
			},
			PersistentVolumeSource: core.PersistentVolumeSource{
				NFS: &core.NFSVolumeSource{
					Server:   p.server,
					Path:     filepath.Join(p.path, dirName),
					ReadOnly: false,
				},
			},
		},
	}
	return pv, controller.ProvisioningFinished, nil
}

func getPersistentVolumeClass(volume *core.PersistentVolume) string {
	// Use beta annotation first
	if class, found := volume.Annotations[core.BetaStorageClassAnnotation]; found {
		return class
	}

	return volume.Spec.StorageClassName
}

func (p *nfsProvisioner) Delete(ctx context.Context, volume *core.PersistentVolume) error {
	path := volume.Spec.PersistentVolumeSource.NFS.Path
	pvName := filepath.Base(path)
	oldPath := filepath.Join(mountPath, pvName)
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		log.Warn().Msgf("path %s does not exist, deletion skipped", oldPath)
		return nil
	}
	// Get the storage class for this volume.
	storageClass, err := p.getClassForVolume(ctx, volume)
	if err != nil {
		return err
	}
	// Determine if the "archiveOnDelete" parameter exists.
	// If it exists and has a false value, delete the directory.
	// Otherwise, archive it.
	archiveOnDelete, exists := storageClass.Parameters["archiveOnDelete"]
	if exists {
		archiveBool, err := strconv.ParseBool(archiveOnDelete)
		if err != nil {
			return err
		}
		if !archiveBool {
			return os.RemoveAll(oldPath)
		}
	}

	archivePath := filepath.Join(mountPath, "archived-"+pvName)
	log.Info().Msgf("archiving path %s to %s", oldPath, archivePath)
	return os.Rename(oldPath, archivePath)

}

// getClassForVolume returns StorageClass
func (p *nfsProvisioner) getClassForVolume(ctx context.Context, pv *core.PersistentVolume) (*storage.StorageClass, error) {
	if p.client == nil {
		return nil, fmt.Errorf("Cannot get kube client")
	}
	className := getPersistentVolumeClass(pv)
	if className == "" {
		return nil, fmt.Errorf("Volume has no storage class")
	}
	class, err := p.client.StorageV1().StorageClasses().Get(ctx, className, meta.GetOptions{})
	if err != nil {
		return nil, err
	}
	return class, nil
}

func main() {
	flag.Parse()
	flag.Set("logtostderr", "true")

	server := os.Getenv("NFS_SERVER")
	if server == "" {
		log.Fatal().Msg("NFS_SERVER not set")
	}
	path := os.Getenv("NFS_PATH")
	if path == "" {
		log.Fatal().Msg("NFS_PATH not set")
	}
	provisionerName := os.Getenv(provisionerNameKey)
	if provisionerName == "" {
		log.Fatal().Msgf("environment variable %s is not set! Please set it.", provisionerNameKey)
	}

	// Create an InClusterConfig and use it to create a client for the controller
	// to use to communicate with Kubernetes
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal().Msgf("Failed to create config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal().Msgf("Failed to create client: %v", err)
	}

	clientNFSProvisioner := &nfsProvisioner{
		client: clientset,
		server: server,
		path:   path,
	}
	// Start the provision controller which will dynamically provision efs NFS
	// PVs
	pc := controller.NewProvisionController(
		clientset,
		provisionerName,
		clientNFSProvisioner,
	)
	pc.Run(context.Background())
}
