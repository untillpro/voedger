/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package vvm

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	istructsmem "github.com/voedger/voedger/pkg/istructsmem"
)

func (hap HVMAppsBuilder) Add(appQName istructs.AppQName, builder HVMAppBuilder) {
	builders := hap[appQName]
	builders = append(builders, builder)
	hap[appQName] = builders
}

func (hap HVMAppsBuilder) PrepareStandardExtensionPoints() map[istructs.AppQName]IStandardExtensionPoints {
	seps := map[istructs.AppQName]IStandardExtensionPoints{}
	for appQName := range hap {
		seps[appQName] = &standardExtensionPointsImpl{rootExtensionPoint: &implIExtensionPoint{}}
	}
	return seps
}

func (hap HVMAppsBuilder) Build(hvmCfg *HVMConfig, cfgs istructsmem.AppConfigsType, hvmAPI HVMAPI, seps map[istructs.AppQName]IStandardExtensionPoints) (hvmApps HVMApps) {
	for appQName, builders := range hap {
		adf := appdef.New()
		sep := seps[appQName]
		cfg := cfgs.AddConfig(appQName, adf)
		for _, builder := range builders {
			builder(hvmCfg, hvmAPI, cfg, adf, sep)
		}
		epPostDocs := sep.ExtensionPoint(EPPostDocs)
		epPostDocs.Iterate(func(eKey EKey, value interface{}) {
			epPostDoc := value.(IExtensionPoint)
			postDocQName := eKey.(appdef.QName)
			postDocDesc := epPostDoc.Value().(PostDocDesc)
			doc := adf.AddStruct(postDocQName, postDocDesc.Kind)
			epPostDoc.Iterate(func(eKey EKey, value interface{}) {
				postDocField := value.(PostDocFieldType)
				if len(postDocField.VerificationKinds) > 0 {
					doc.AddVerifiedField(eKey.(string), postDocField.Kind, postDocField.Required, postDocField.VerificationKinds...)
				} else {
					doc.AddField(eKey.(string), postDocField.Kind, postDocField.Required)
				}
			})
			if postDocDesc.IsSingleton {
				doc.SetSingleton()
			}
		})
		hvmApps = append(hvmApps, appQName)
		// TODO: remove it after https://github.com/voedger/voedger/issues/56
		if _, err := adf.Build(); err != nil {
			panic(err)
		}
	}
	return hvmApps
}

func (ar *standardExtensionPointsImpl) ExtensionPoint(epKey EPKey) IExtensionPoint {
	return ar.rootExtensionPoint.ExtensionPoint(epKey)
}
func (ar *standardExtensionPointsImpl) EPWSTemplates() IEPWSTemplates {
	return ar.rootExtensionPoint.ExtensionPoint(EPWSTemplates)
}
func (ar *standardExtensionPointsImpl) EPJournalIndices() IEPJournalIndices {
	return ar.rootExtensionPoint.ExtensionPoint(EPJournalIndices)
}
func (ar *standardExtensionPointsImpl) EPJournalPredicates() IEPJournalPredicates {
	return ar.rootExtensionPoint.ExtensionPoint(EPJournalPredicates)
}
