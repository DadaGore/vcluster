package storageclasses

import (
	"testing"

	"github.com/loft-sh/vcluster/pkg/config"
	synccontext "github.com/loft-sh/vcluster/pkg/controllers/syncer/context"
	testingutil "github.com/loft-sh/vcluster/pkg/util/testing"
	"github.com/loft-sh/vcluster/pkg/util/translate"
	"gotest.tools/assert"

	generictesting "github.com/loft-sh/vcluster/pkg/controllers/syncer/testing"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSync(t *testing.T) {
	translate.Default = translate.NewSingleNamespaceTranslator(generictesting.DefaultTestTargetNamespace)

	vObjectMeta := metav1.ObjectMeta{
		Name:            "testsc",
		ResourceVersion: generictesting.FakeClientResourceVersion,
	}
	vObject := &storagev1.StorageClass{
		ObjectMeta:  vObjectMeta,
		Provisioner: "my-provisioner",
	}
	pObject := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:            translate.Default.PhysicalNameClusterScoped(vObjectMeta.Name),
			ResourceVersion: generictesting.FakeClientResourceVersion,
			Labels: map[string]string{
				translate.MarkerLabel: translate.VClusterName,
			},
			Annotations: map[string]string{
				translate.NameAnnotation: "testsc",
				translate.UIDAnnotation:  "",
				translate.KindAnnotation: storagev1.SchemeGroupVersion.WithKind("StorageClass").String(),
			},
		},
		Provisioner: "my-provisioner",
	}
	vObjectUpdated := &storagev1.StorageClass{
		ObjectMeta:  vObjectMeta,
		Provisioner: "my-provisioner",
		Parameters: map[string]string{
			"TEST": "TEST",
		},
	}
	pObjectUpdated := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: translate.Default.PhysicalNameClusterScoped(vObjectMeta.Name),
			Labels: map[string]string{
				translate.MarkerLabel: translate.VClusterName,
			},
			Annotations: map[string]string{
				translate.NameAnnotation: "testsc",
				translate.UIDAnnotation:  "",
				translate.KindAnnotation: storagev1.SchemeGroupVersion.WithKind("StorageClass").String(),
			},
		},
		Provisioner: "my-provisioner",
		Parameters: map[string]string{
			"TEST": "TEST",
		},
	}

	generictesting.RunTestsWithContext(t, func(vConfig *config.VirtualClusterConfig, pClient *testingutil.FakeIndexClient, vClient *testingutil.FakeIndexClient) *synccontext.RegisterContext {
		vConfig.Sync.ToHost.StorageClasses.Enabled = true
		return generictesting.NewFakeRegisterContext(vConfig, pClient, vClient)
	}, []*generictesting.SyncTest{
		{
			Name:                "Sync Down",
			InitialVirtualState: []runtime.Object{vObject},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				storagev1.SchemeGroupVersion.WithKind("StorageClass"): {vObject},
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				storagev1.SchemeGroupVersion.WithKind("StorageClass"): {pObject},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, New)
				_, err := syncer.(*storageClassSyncer).SyncToHost(syncCtx, vObject.DeepCopy())
				assert.NilError(t, err)
			},
		},
		{
			Name:                 "Sync",
			InitialVirtualState:  []runtime.Object{vObjectUpdated.DeepCopy()},
			InitialPhysicalState: []runtime.Object{pObject.DeepCopy()},
			ExpectedVirtualState: map[schema.GroupVersionKind][]runtime.Object{
				storagev1.SchemeGroupVersion.WithKind("StorageClass"): {vObjectUpdated.DeepCopy()},
			},
			ExpectedPhysicalState: map[schema.GroupVersionKind][]runtime.Object{
				storagev1.SchemeGroupVersion.WithKind("StorageClass"): {pObjectUpdated.DeepCopy()},
			},
			Sync: func(ctx *synccontext.RegisterContext) {
				syncCtx, syncer := generictesting.FakeStartSyncer(t, ctx, New)
				_, err := syncer.(*storageClassSyncer).Sync(syncCtx, pObject.DeepCopy(), vObjectUpdated.DeepCopy())
				assert.NilError(t, err)
			},
		},
	})
}
