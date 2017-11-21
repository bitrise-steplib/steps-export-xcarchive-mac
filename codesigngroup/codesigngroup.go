package codesigngroup

import (
	"github.com/bitrise-tools/go-xcode/certificateutil"
	"github.com/bitrise-tools/go-xcode/export"
	"github.com/bitrise-tools/go-xcode/exportoptions"
	"github.com/bitrise-tools/go-xcode/plistutil"
	"github.com/bitrise-tools/go-xcode/profileutil"
)

// GroupModel ...
type GroupModel struct {
	Groups              []export.SelectableCodeSignGroup
	InstalledIdentities InstalledIdentitiesModel
}

// InstalledIdentitiesModel ...
type InstalledIdentitiesModel struct {
	Certificates []certificateutil.CertificateInfoModel
	Profiles     []profileutil.ProvisioningProfileInfoModel
}

// Filter ...
type Filter struct {
	model *GroupModel
}

// New ...
func New(bundleIDs []string, profileType profileutil.ProfileType) (*GroupModel, error) {
	installedCertificates, err := certificateutil.InstalledCodesigningCertificateInfos()
	if err != nil {
		return nil, err
	}

	installedProfiles, err := profileutil.InstalledProvisioningProfileInfos(profileType)
	if err != nil {
		return nil, err
	}

	return &GroupModel{
		Groups: export.CreateSelectableCodeSignGroups(installedCertificates, installedProfiles, bundleIDs),
		InstalledIdentities: InstalledIdentitiesModel{
			Certificates: installedCertificates,
			Profiles:     installedProfiles,
		},
	}, nil
}

// Filter ...
func (groupModel *GroupModel) Filter() *Filter {
	return &Filter{model: groupModel}
}

// ByMethod ...
func (filter *Filter) ByMethod(method exportoptions.Method) *Filter {
	filter.model.Groups = export.FilterSelectableCodeSignGroups(filter.model.Groups,
		export.CreateExportMethodSelectableCodeSignGroupFilter(method),
	)

	return filter
}

// ByEntitlements ...
func (filter *Filter) ByEntitlements(entitlements map[string]plistutil.PlistData) *Filter {
	filter.model.Groups = export.FilterSelectableCodeSignGroups(filter.model.Groups,
		export.CreateEntitlementsSelectableCodeSignGroupFilter(entitlements),
	)
	return filter
}
