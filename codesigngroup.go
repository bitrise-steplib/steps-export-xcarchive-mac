package main

import (
	"fmt"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-xcode/certificateutil"
	"github.com/bitrise-io/go-xcode/export"
	"github.com/bitrise-io/go-xcode/exportoptions"
	"github.com/bitrise-io/go-xcode/profileutil"
	"github.com/bitrise-io/go-xcode/xcarchive"
)

func generateMacExportOptionsPlist(archive xcarchive.MacosArchive, exportMethod exportoptions.Method, teamID string) (exportoptions.ExportOptions, error) {
	var macCodeSignGroup *export.MacCodeSignGroup
	exportProfileMapping := map[string]string{}

	// We do not need provisioning profile for the export if the app in the generated XcArchive doesn't
	// contain embedded provisioning profile.
	if archive.Application.ProvisioningProfile != nil {
		installedCertificates, err := certificateutil.InstalledCodesigningCertificateInfos()
		if err != nil {
			return nil, fmt.Errorf("failed to get installed certificates: %s", err)
		}
		certificates := certificateutil.FilterValidCertificateInfos(installedCertificates)
		validCertificates := append(certificates.ValidCertificates, certificates.DuplicatedCertificates...)

		log.Debugf("\n")
		log.Debugf("Installed valid certificates:")
		for _, certInfo := range validCertificates {
			log.Debugf(certInfo.String())
		}

		log.Debugf("\n")
		log.Debugf("Installed invalid certificates:")
		for _, certInfo := range certificates.InvalidCertificates {
			log.Debugf(certInfo.String())
		}
		log.Debugf("\n")

		installedProfiles, err := profileutil.InstalledProvisioningProfileInfos(profileutil.ProfileTypeMacOs)
		if err != nil {
			return nil, fmt.Errorf("failed to get installed provisioning profiles: %s", err)
		}

		log.Debugf("\n")
		log.Debugf("Installed profiles:")
		for _, profInfo := range installedProfiles {
			log.Debugf(profInfo.String())
		}

		var validInstallerCertificates []certificateutil.CertificateInfoModel
		if exportMethod == exportoptions.MethodAppStore {
			installedInstallerCertificates, err := certificateutil.InstalledInstallerCertificateInfos()
			if err != nil {
				log.Errorf("Failed to read installed Installer certificates, error: %s", err)
			}
			installerCertificates := certificateutil.FilterValidCertificateInfos(installedInstallerCertificates)
			validInstallerCertificates = append(installerCertificates.ValidCertificates, installerCertificates.DuplicatedCertificates...)

			log.Debugf("\n")
			log.Debugf("Installed valid installer certificates:")
			for _, certInfo := range validInstallerCertificates {
				log.Debugf(certInfo.String())
			}

			log.Debugf("\n")
			log.Debugf("Installed invalid installer certificates:")
			for _, certInfo := range installerCertificates.InvalidCertificates {
				log.Debugf(certInfo.String())
			}
		}

		macCodeSignGroup, err = matchingMacCodeSignGroups(archive, validCertificates, validInstallerCertificates, installedProfiles, exportMethod, teamID)
		if err != nil {
			return nil, fmt.Errorf("failed to find code sign groups for the project: %s", err)
		}

		if macCodeSignGroup != nil {
			for bundleID, profileInfo := range macCodeSignGroup.BundleIDProfileMap() {
				exportProfileMapping[bundleID] = profileInfo.Name
			}
		}

	} else {
		log.Printf("Archive was generated without provisioning profile.")
		log.Printf("Export the application using automatic signing...")
		fmt.Println()
	}

	var exportOpts exportoptions.ExportOptions
	if exportMethod == exportoptions.MethodAppStore {
		options := exportoptions.NewAppStoreOptions()

		if macCodeSignGroup != nil {
			options.BundleIDProvisioningProfileMapping = exportProfileMapping
			options.SigningCertificate = macCodeSignGroup.Certificate().CommonName
			options.InstallerSigningCertificate = macCodeSignGroup.InstallerCertificate().CommonName
		}

		exportOpts = options
	} else {
		options := exportoptions.NewNonAppStoreOptions(exportMethod)

		if macCodeSignGroup != nil {
			options.BundleIDProvisioningProfileMapping = exportProfileMapping
			options.SigningCertificate = macCodeSignGroup.Certificate().CommonName
		}

		exportOpts = options
	}

	return exportOpts, nil
}

func matchingMacCodeSignGroups(archive xcarchive.MacosArchive, installedCertificates []certificateutil.CertificateInfoModel,
	installedInstallerCertificates []certificateutil.CertificateInfoModel, installedProfiles []profileutil.ProvisioningProfileInfoModel,
	exportMethod exportoptions.Method, forceTeamID string) (*export.MacCodeSignGroup, error) {
	if archive.Application.ProvisioningProfile == nil {
		return nil, fmt.Errorf("precondition false, provisioning profile expected in the archive")
	}

	log.Printf("Bundle ID to Entitlements mapping")
	bundleIDEntitlementsMap := archive.BundleIDEntitlementsMap()
	bundleIDs := []string{}
	for bundleID, entitlements := range bundleIDEntitlementsMap {
		bundleIDs = append(bundleIDs, bundleID)

		entitlementKeys := []string{}
		for key := range entitlements {
			entitlementKeys = append(entitlementKeys, key)
		}
		log.Debugf("- %s entitlements: %s", bundleID, entitlementKeys)
	}

	log.Infof("Resolving CodeSignGroups...")
	codeSignGroups := export.CreateSelectableCodeSignGroups(installedCertificates, installedProfiles, bundleIDs)
	if len(codeSignGroups) == 0 {
		log.Errorf("Failed to find code signing groups for specified export method (%s)", exportMethod)
	}

	log.Debugf("\nGroups:")
	for _, group := range codeSignGroups {
		log.Debugf(group.String())
	}

	if len(bundleIDEntitlementsMap) > 0 {
		log.Warnf("Filtering CodeSignInfo groups for target capabilities")

		codeSignGroups = export.FilterSelectableCodeSignGroups(codeSignGroups, export.CreateEntitlementsSelectableCodeSignGroupFilter(bundleIDEntitlementsMap))

		log.Debugf("\nGroups after filtering for target capabilities:")
		for _, group := range codeSignGroups {
			log.Debugf(group.String())
		}
	}

	log.Warnf("Filtering CodeSignInfo groups for export method")

	codeSignGroups = export.FilterSelectableCodeSignGroups(codeSignGroups, export.CreateExportMethodSelectableCodeSignGroupFilter(exportMethod))

	log.Debugf("\nGroups after filtering for export method:")
	for _, group := range codeSignGroups {
		log.Debugf(group.String())
	}

	if forceTeamID != "" {
		log.Warnf("Export TeamID specified: %s, filtering CodeSignInfo groups...", forceTeamID)

		codeSignGroups = export.FilterSelectableCodeSignGroups(codeSignGroups, export.CreateTeamSelectableCodeSignGroupFilter(forceTeamID))

		log.Debugf("\nGroups after filtering for team ID:")
		for _, group := range codeSignGroups {
			log.Debugf(group.String())
		}
	}

	log.Debugf("Provisioning profile name in the archive: %s", archive.Application.ProvisioningProfile.Name)
	if !archive.IsXcodeManaged() {
		log.Warnf("App was signed with NON xcode managed profile when archiving,\n" +
			"only NOT xcode managed profiles are allowed to sign when exporting the archive.\n" +
			"Removing xcode managed CodeSignInfo groups")

		codeSignGroups = export.FilterSelectableCodeSignGroups(codeSignGroups, export.CreateNotXcodeManagedSelectableCodeSignGroupFilter())

		log.Debugf("\nGroups after filtering for NOT Xcode managed profiles:")
		for _, group := range codeSignGroups {
			log.Debugf(group.String())
		}
	}

	macCodeSignGroups := export.CreateMacCodeSignGroup(codeSignGroups, installedInstallerCertificates, exportMethod)
	if len(macCodeSignGroups) == 0 {
		return nil, fmt.Errorf("Can not create macos codesiging groups for the project")
	} else if len(macCodeSignGroups) > 1 {
		log.Warnf("Multiple matching  codesiging groups found for the project, using first...")
	}
	return &(macCodeSignGroups[0]), nil
}
