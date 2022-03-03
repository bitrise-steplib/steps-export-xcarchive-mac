package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-xcode/exportoptions"
	"github.com/bitrise-io/go-xcode/utility"
	"github.com/bitrise-io/go-xcode/xcarchive"
	"github.com/bitrise-io/go-xcode/xcodebuild"
	"github.com/bitrise-steplib/steps-export-xcarchive-mac/utils"
)

const (
	bitriseAppPathEnvKey                = "BITRISE_APP_PATH"
	bitrisePKGPathEnvKey                = "BITRISE_PKG_PATH"
	bitriseIDEDistributionLogsPthEnvKey = "BITRISE_IDEDISTRIBUTION_LOGS_PATH"
)

// ConfigsModel ...
type ConfigsModel struct {
	ArchivePath string

	ExportMethod                    string
	UploadBitcode                   string
	CompileBitcode                  string
	TeamID                          string
	CustomExportOptionsPlistContent string

	UseLegacyExport                     string
	LegacyExportProvisioningProfileName string
	LegacyExportOutputFormat            string

	VerboseLog string
	DeployDir  string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		ArchivePath: os.Getenv("archive_path"),

		ExportMethod:                    os.Getenv("export_method"),
		UploadBitcode:                   os.Getenv("upload_bitcode"),
		CompileBitcode:                  os.Getenv("compile_bitcode"),
		TeamID:                          os.Getenv("team_id"),
		CustomExportOptionsPlistContent: os.Getenv("custom_export_options_plist_content"),

		UseLegacyExport:                     os.Getenv("use_legacy_export"),
		LegacyExportProvisioningProfileName: os.Getenv("legacy_export_provisioning_profile_name"),
		LegacyExportOutputFormat:            os.Getenv("legacy_export_output_format"),

		DeployDir:  os.Getenv("BITRISE_DEPLOY_DIR"),
		VerboseLog: os.Getenv("verbose_log"),
	}
}

func (configs ConfigsModel) print() {
	log.Infof("Configs:")
	log.Printf("- ArchivePath: %s", configs.ArchivePath)
	log.Printf("- ExportMethod: %s", configs.ExportMethod)
	log.Printf("- UploadBitcode: %s", configs.UploadBitcode)
	log.Printf("- CompileBitcode: %s", configs.CompileBitcode)
	log.Printf("- TeamID: %s", configs.TeamID)
	log.Printf("- VerboseLog: %s", configs.VerboseLog)

	log.Infof("Experimental Configs:")
	log.Printf("- UseLegacyExport: %s", configs.UseLegacyExport)
	log.Printf("- LegacyExportProvisioningProfileName: %s", configs.LegacyExportProvisioningProfileName)
	log.Printf("- LegacyExportOutputFormat: %s", configs.LegacyExportOutputFormat)
	log.Printf("- CustomExportOptionsPlistContent:")
	if configs.CustomExportOptionsPlistContent != "" {
		fmt.Println(configs.CustomExportOptionsPlistContent)
	}

	log.Infof("Other Configs:")
	log.Printf("- DeployDir: %s", configs.DeployDir)
}

func (configs ConfigsModel) validate() error {
	if configs.ArchivePath == "" {
		return errors.New("no ArchivePath specified")
	}

	if exist, err := pathutil.IsPathExists(configs.ArchivePath); err != nil {
		return fmt.Errorf("failed to check if ArchivePath exist at: %s, error: %s", configs.ArchivePath, err)
	} else if !exist {
		return fmt.Errorf("ArchivePath not exist at: %s", configs.ArchivePath)
	}

	if configs.ExportMethod == "" {
		return errors.New("no ExportMethod specified")
	}
	if configs.UploadBitcode == "" {
		return errors.New("no UploadBitcode specified")
	}
	if configs.CompileBitcode == "" {
		return errors.New("no CompileBitcode specified")
	}

	if configs.UseLegacyExport == "" {
		return errors.New("no UseLegacyExport specified")
	}
	if configs.LegacyExportOutputFormat == "" {
		return errors.New("no LegacyExportOutputFormat specified")
	}

	return nil
}

func fail(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

func findIDEDistrubutionLogsPath(output string) (string, error) {
	pattern := `IDEDistribution: -\[IDEDistributionLogging _createLoggingBundleAtPath:\]: Created bundle at path "(?P<log_path>.*)"`
	re := regexp.MustCompile(pattern)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if match := re.FindStringSubmatch(line); len(match) == 2 {
			return match[1], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", nil
}

func main() {
	configs := createConfigsModelFromEnvs()

	fmt.Println()
	configs.print()

	if err := configs.validate(); err != nil {
		fail("Issue with input: %s", err)
	}

	log.SetEnableDebugLog(configs.VerboseLog == "yes")

	archiveExt := filepath.Ext(configs.ArchivePath)
	archiveName := filepath.Base(configs.ArchivePath)
	archiveName = strings.TrimSuffix(archiveName, archiveExt)
	appPath := filepath.Join(configs.DeployDir, archiveName+".app")
	pkgPath := filepath.Join(configs.DeployDir, archiveName+".pkg")
	exportOptionsPath := filepath.Join(configs.DeployDir, "export_options.plist")
	ideDistributionLogsZipPath := filepath.Join(configs.DeployDir, "xcodebuild.xcdistributionlogs.zip")

	xcodebuildVersion, err := utility.GetXcodeVersion()
	if err != nil {
		fail("Failed to determine xcode version, error: %s", err)
	}
	log.Printf("- xcodebuildVersion: %s (%s)", xcodebuildVersion.Version, xcodebuildVersion.BuildVersion)

	customExportOptionsPlistContent := strings.TrimSpace(configs.CustomExportOptionsPlistContent)
	if customExportOptionsPlistContent != configs.CustomExportOptionsPlistContent {
		fmt.Println()
		log.Warnf("CustomExportOptionsPlistContent is stripped to remove spaces and new lines:")
		log.Printf(customExportOptionsPlistContent)
	}

	envsToUnset := []string{"GEM_HOME", "GEM_PATH", "RUBYLIB", "RUBYOPT", "BUNDLE_BIN_PATH", "_ORIGINAL_GEM_PATH", "BUNDLE_GEMFILE"}
	for _, key := range envsToUnset {
		if err := os.Unsetenv(key); err != nil {
			fail("Failed to unset (%s), error: %s", key, err)
		}
	}

	archive, err := xcarchive.NewMacosArchive(configs.ArchivePath)
	if err != nil {
		fail("Failed to parse archive, error: %s", err)
	}

	// do a simple export if method set to none
	{
		if configs.ExportMethod == "none" {
			log.Infof("Exporting app without re-sign...")

			if err := utils.ExportAppFromArchive(configs.ArchivePath, configs.DeployDir, bitriseAppPathEnvKey); err != nil {
				fail("Failed to archive app, error: %s", err)
			}

			log.Donef("The app path is now available in the Environment Variable: %s", bitriseAppPathEnvKey)
			return
		}
	}

	exportMethod, err := exportoptions.ParseMethod(configs.ExportMethod)
	if err != nil {
		fail("Failed to parse export options, error: %s", err)
	}

	// legacy export
	{
		if configs.UseLegacyExport == "yes" {
			log.Infof("Using legacy export method...")

			if xcodebuildVersion.MajorVersion >= 9 {
				fail("Legacy export method (using '-exportFormat ipa' flag) is not supported from Xcode version 9")
			}

			provisioningProfileName := ""
			if configs.LegacyExportProvisioningProfileName != "" {
				log.Printf("Using provisioning profile: %s", configs.LegacyExportProvisioningProfileName)

				provisioningProfileName = configs.LegacyExportProvisioningProfileName
			} else {
				log.Printf("Using embedded provisioning profile")

				if archive.Application.ProvisioningProfile == nil {
					fail("No embedded.provisionprofile found nor Provisioning Profile name to use by export specified")
				}

				provisioningProfileName = archive.Application.ProvisioningProfile.Name
				log.Printf("embedded profile name: %s", provisioningProfileName)
			}

			exportingApp := true
			if configs.LegacyExportOutputFormat == "pkg" {
				exportingApp = false
			}

			legacyExportCmd := xcodebuild.NewLegacyExportCommand()
			legacyExportCmd.SetExportFormat(configs.LegacyExportOutputFormat)

			if exportingApp {
				legacyExportCmd.SetExportPath(appPath)
			} else {
				legacyExportCmd.SetExportPath(pkgPath)
			}

			legacyExportCmd.SetArchivePath(configs.ArchivePath)
			legacyExportCmd.SetExportProvisioningProfileName(provisioningProfileName)

			log.Donef("$ %s", legacyExportCmd.PrintableCmd())
			fmt.Println()

			if err := legacyExportCmd.Run(); err != nil {
				fail("Export failed, error: %s", err)
			}

			if exportingApp {
				if err := utils.ExportOutputFile(appPath, appPath, bitriseAppPathEnvKey); err != nil {
					fail("Failed to export %s, error: %s", bitriseAppPathEnvKey, err)
				}

				log.Donef("The app path is now available in the Environment Variable: %s (value: %s)", bitriseAppPathEnvKey, appPath)
			} else {
				if err := utils.ExportOutputFile(pkgPath, pkgPath, bitrisePKGPathEnvKey); err != nil {
					fail("Failed to export %s, error: %s", bitrisePKGPathEnvKey, err)
				}

				log.Donef("The pkg path is now available in the Environment Variable: %s (value: %s)", bitrisePKGPathEnvKey, appPath)
			}

			return
		}
	}

	log.Infof("Exporting with export options...")

	exportOptionsPlistContent := ""

	if customExportOptionsPlistContent != "" {
		log.Printf("Custom export options content provided:")
		fmt.Println(customExportOptionsPlistContent)

		exportOptionsPlistContent = customExportOptionsPlistContent
	}

	if exportOptionsPlistContent == "" {
		log.Printf("Generating export options")

		if xcodebuildVersion.MajorVersion >= 9 {
			log.Printf("xcode major version > 9, generating provisioningProfiles node")

			exportOpts, err := generateMacExportOptionsPlist(archive, exportMethod, configs.TeamID)
			if err != nil {
				fail("Export options could not be generated: %v", err)
			}

			log.Printf("generated export options content:")
			fmt.Println()
			fmt.Println(exportOpts.String())

			exportOptionsPlistContent, err = exportOpts.String()
			if err != nil {
				fail("Failed to get exportOptions, error: %s", err)
			}
		}
	}

	if err := fileutil.WriteStringToFile(exportOptionsPath, exportOptionsPlistContent); err != nil {
		fail("Failed to write export options to file, error: %s", err)
	}

	fmt.Println()

	tmpDir, err := pathutil.NormalizedOSTempDirPath("__export__")
	if err != nil {
		fail("Failed to create tmp dir, error: %s", err)
	}

	exportCmd := xcodebuild.NewExportCommand()
	exportCmd.SetArchivePath(configs.ArchivePath)
	exportCmd.SetExportDir(tmpDir)
	exportCmd.SetExportOptionsPlist(exportOptionsPath)

	log.Donef("$ %s", exportCmd.PrintableCmd())
	fmt.Println()

	if xcodebuildOut, err := exportCmd.RunAndReturnOutput(); err != nil {
		// xcdistributionlogs
		if logsDirPth, err := findIDEDistrubutionLogsPath(xcodebuildOut); err != nil {
			log.Warnf("Failed to find xcdistributionlogs, error: %s", err)
		} else if err := utils.ExportOutputDirAsZip(logsDirPth, ideDistributionLogsZipPath, bitriseIDEDistributionLogsPthEnvKey); err != nil {
			log.Warnf("Failed to export %s, error: %s", bitriseIDEDistributionLogsPthEnvKey, err)
		} else {
			log.Warnf(`If you can't find the reason of the error in the log, please check the xcdistributionlogs
The logs directory is stored in $BITRISE_DEPLOY_DIR, and its full path
is available in the $BITRISE_IDEDISTRIBUTION_LOGS_PATH environment variable`)
		}

		fail("Export failed, error: %s", err)
	}

	pattern := filepath.Join(tmpDir, "*.app")
	apps, err := filepath.Glob(pattern)
	if err != nil {
		fail("Failed to find app, with pattern: %s, error: %s", pattern, err)
	}

	if len(apps) > 0 {
		if err := utils.ExportOutputDirAsZip(apps[0], appPath, bitriseAppPathEnvKey); err != nil {
			fail("Failed to export %s, error: %s", bitriseAppPathEnvKey, err)
		}

		log.Donef("The app path is now available in the Environment Variable: %s (value: %s)", bitriseAppPathEnvKey, appPath)
		return
	}

	pattern = filepath.Join(tmpDir, "*.pkg")
	pkgs, err := filepath.Glob(pattern)
	if err != nil {
		fail("Failed to find pkg, with pattern: %s, error: %s", pattern, err)
	}

	if len(pkgs) > 0 {
		if err := utils.ExportOutputFile(pkgs[0], pkgPath, bitrisePKGPathEnvKey); err != nil {
			fail("Failed to export %s, error: %s", bitrisePKGPathEnvKey, err)
		}

		log.Donef("The pkg path is now available in the Environment Variable: %s (value: %s)", bitrisePKGPathEnvKey, appPath)
	} else {
		fail("No app nor pkg output generated")
	}
}
