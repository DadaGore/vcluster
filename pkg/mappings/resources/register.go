package resources

import (
	"fmt"

	synccontext "github.com/loft-sh/vcluster/pkg/controllers/syncer/context"
	"github.com/loft-sh/vcluster/pkg/mappings"
)

// ExtraMappers that will be started as well
var ExtraMappers []BuildMapper

// BuildMapper is a function to build a new mapper
type BuildMapper func(ctx *synccontext.RegisterContext) (mappings.Mapper, error)

func getMappers(ctx *synccontext.RegisterContext) []BuildMapper {
	return append([]BuildMapper{
		CreateSecretsMapper,
		CreateConfigMapsMapper,
		isEnabled(ctx.Config.Sync.FromHost.CSINodes.Enabled == "true", CreateCSINodesMapper),
		isEnabled(ctx.Config.Sync.FromHost.CSIDrivers.Enabled == "true", CreateCSIDriversMapper),
		isEnabled(ctx.Config.Sync.FromHost.CSIStorageCapacities.Enabled == "true", CreateCSIStorageCapacitiesMapper),
		CreateEndpointsMapper,
		CreateEventsMapper,
		CreateIngressClassesMapper,
		CreateIngressesMapper,
		CreateNamespacesMapper,
		CreateNetworkPoliciesMapper,
		CreateNodesMapper,
		CreatePersistentVolumeClaimsMapper,
		CreateServiceAccountsMapper,
		CreateServiceMapper,
		CreatePriorityClassesMapper,
		CreatePodDisruptionBudgetsMapper,
		CreatePersistentVolumesMapper,
		CreatePodsMapper,
		CreateStorageClassesMapper,
		CreateVolumeSnapshotClassesMapper,
		CreateVolumeSnapshotContentsMapper,
		CreateVolumeSnapshotsMapper,
	}, ExtraMappers...)
}

func MustRegisterMappings(ctx *synccontext.RegisterContext) {
	err := RegisterMappings(ctx)
	if err != nil {
		panic(err.Error())
	}
}

func RegisterMappings(ctx *synccontext.RegisterContext) error {
	// create mappers
	for _, createFunc := range getMappers(ctx) {
		if createFunc == nil {
			continue
		}

		mapper, err := createFunc(ctx)
		if err != nil {
			return fmt.Errorf("create mapper: %w", err)
		} else if mapper == nil {
			continue
		}

		err = mappings.Default.AddMapper(mapper)
		if err != nil {
			return fmt.Errorf("add mapper %s: %w", mapper.GroupVersionKind().String(), err)
		}
	}

	return nil
}

func isEnabled[T any](enabled bool, fn T) T {
	if enabled {
		return fn
	}
	var ret T
	return ret
}
