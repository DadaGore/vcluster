package volumesnapshotcontents

import (
	"context"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/loft-sh/vcluster/pkg/constants"
	"github.com/loft-sh/vcluster/pkg/mappings"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (s *volumeSnapshotContentSyncer) translate(ctx context.Context, vVSC *volumesnapshotv1.VolumeSnapshotContent) *volumesnapshotv1.VolumeSnapshotContent {
	pVSC := s.TranslateMetadata(ctx, vVSC).(*volumesnapshotv1.VolumeSnapshotContent)
	pVolumeSnapshot := mappings.VirtualToHost(ctx, vVSC.Spec.VolumeSnapshotRef.Name, vVSC.Spec.VolumeSnapshotRef.Namespace, mappings.VolumeSnapshots())
	pVSC.Spec.VolumeSnapshotRef = corev1.ObjectReference{
		Namespace: pVolumeSnapshot.Namespace,
		Name:      pVolumeSnapshot.Name,
	}
	return pVSC
}

func (s *volumeSnapshotContentSyncer) translateBackwards(pVSC *volumesnapshotv1.VolumeSnapshotContent, vVS *volumesnapshotv1.VolumeSnapshot) *volumesnapshotv1.VolumeSnapshotContent {
	// build virtual VolumeSnapshotContent object
	vObj := pVSC.DeepCopy()
	vObj.ResourceVersion = ""
	vObj.UID = ""
	vObj.ManagedFields = nil

	if vVS != nil {
		vObj.Spec.VolumeSnapshotRef = translateVolumeSnapshotRefBackwards(&vObj.Spec.VolumeSnapshotRef, vVS)
	}

	if vObj.Annotations == nil {
		vObj.Annotations = map[string]string{}
	}
	vObj.Annotations[constants.HostClusterVSCAnnotation] = pVSC.Name

	return vObj
}

func (s *volumeSnapshotContentSyncer) translateUpdateBackwards(pVSC, vVSC *volumesnapshotv1.VolumeSnapshotContent) {
	// add a finalizer to ensure that we delete the physical VolumeSnapshotContent object when virtual is being deleted
	pCopy := pVSC.DeepCopy()
	if pCopy.Finalizers == nil {
		pCopy.Finalizers = []string{}
	}
	controllerutil.AddFinalizer(pCopy, PhysicalVSCGarbageCollectionFinalizer)
	vVSC.Finalizers = pCopy.Finalizers
}

func translateVolumeSnapshotRefBackwards(ref *corev1.ObjectReference, vVS *volumesnapshotv1.VolumeSnapshot) corev1.ObjectReference {
	newRef := ref.DeepCopy()
	newRef.Namespace = vVS.Namespace
	newRef.Name = vVS.Name
	newRef.UID = vVS.UID
	newRef.ResourceVersion = vVS.ResourceVersion
	return *newRef
}
