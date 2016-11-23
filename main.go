package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bitrise-io/go-utils/cmdex"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-steplib/steps-export-xcarchive-mac/utils"
	"github.com/bitrise-tools/go-xcode/exportoptions"
	"github.com/bitrise-tools/go-xcode/xcodebuild"
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

	DeployDir string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		ArchivePath: os.Getenv("archive_path"),

		ExportMethod:   os.Getenv("export_method"),
		UploadBitcode:  os.Getenv("upload_bitcode"),
		CompileBitcode: os.Getenv("compile_bitcode"),
		TeamID:         os.Getenv("team_id"),
		CustomExportOptionsPlistContent: os.Getenv("custom_export_options_plist_content"),

		UseLegacyExport:                     os.Getenv("use_legacy_export"),
		LegacyExportProvisioningProfileName: os.Getenv("legacy_export_provisioning_profile_name"),
		LegacyExportOutputFormat:            os.Getenv("legacy_export_output_format"),

		DeployDir: os.Getenv("BITRISE_DEPLOY_DIR"),
	}
}

func (configs ConfigsModel) print() {
	log.Info("Configs:")
	log.Detail("- ArchivePath: %s", configs.ArchivePath)
	log.Detail("- ExportMethod: %s", configs.ExportMethod)
	log.Detail("- UploadBitcode: %s", configs.UploadBitcode)
	log.Detail("- CompileBitcode: %s", configs.CompileBitcode)
	log.Detail("- TeamID: %s", configs.TeamID)

	log.Info("Experimental Configs:")
	log.Detail("- UseLegacyExport: %s", configs.UseLegacyExport)
	log.Detail("- LegacyExportProvisioningProfileName: %s", configs.LegacyExportProvisioningProfileName)
	log.Detail("- LegacyExportOutputFormat: %s", configs.LegacyExportOutputFormat)
	log.Detail("- CustomExportOptionsPlistContent:")
	if configs.CustomExportOptionsPlistContent != "" {
		fmt.Println(configs.CustomExportOptionsPlistContent)
	}

	log.Info("Other Configs:")
	log.Detail("- DeployDir: %s", configs.DeployDir)
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
	log.Error(format, v...)
	os.Exit(1)
}

func isToolInstalled(name string) bool {
	cmd := cmdex.NewCommand("which", name)
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	return err == nil && out != ""
}

func applyRVMFix() error {
	if !isToolInstalled("rvm") {
		return nil
	}
	log.Warn(`Applying RVM 'fix'`)

	homeDir := pathutil.UserHomeDir()
	rvmScriptPth := filepath.Join(homeDir, ".rvm/scripts/rvm")
	if exist, err := pathutil.IsPathExists(rvmScriptPth); err != nil {
		return err
	} else if !exist {
		return nil
	}

	if err := cmdex.NewCommand("source", rvmScriptPth).Run(); err != nil {
		return err
	}

	if err := cmdex.NewCommand("rvm", "use", "system").Run(); err != nil {
		return err
	}

	return nil
}

func findIDEDistrubutionLogsPath(output string) (string, error) {
	pattern := `IDEDistribution: -\[IDEDistributionLogging _createLoggingBundleAtPath:\]: Created bundle at path '(?P<log_path>.*)'`
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
		fmt.Println()
		fail("Issue with input: %s", err)

	}
	fmt.Println()

	archiveExt := filepath.Ext(configs.ArchivePath)
	archiveName := filepath.Base(configs.ArchivePath)
	archiveName = strings.TrimSuffix(archiveName, archiveExt)

	appPath := filepath.Join(configs.DeployDir, archiveName+".app")
	pkgPath := filepath.Join(configs.DeployDir, archiveName+".pkg")
	exportOptionsPath := filepath.Join(configs.DeployDir, "export_options.plist")

	ideDistributionLogsZipPath := filepath.Join(configs.DeployDir, "xcodebuild.xcdistributionlogs.zip")

	envsToUnset := []string{"GEM_HOME", "GEM_PATH", "RUBYLIB", "RUBYOPT", "BUNDLE_BIN_PATH", "_ORIGINAL_GEM_PATH", "BUNDLE_GEMFILE"}
	for _, key := range envsToUnset {
		if err := os.Unsetenv(key); err != nil {
			fail("Failed to unset (%s), error: %s", key, err)
		}
	}

	if configs.UseLegacyExport == "yes" {
		log.Info("Using legacy export method...")

		provisioningProfileName := ""
		if configs.LegacyExportProvisioningProfileName != "" {
			log.Detail("Using provisioning profile: %s", configs.LegacyExportProvisioningProfileName)
			provisioningProfileName = configs.LegacyExportProvisioningProfileName
		} else {
			log.Detail("No provisining profile specified, let xcodebuild to grab one...")
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

		log.Done("$ %s", legacyExportCmd.PrintableCmd())
		fmt.Println()

		if err := legacyExportCmd.Run(); err != nil {
			fail("Export failed, error: %s", err)
		}

		if exportingApp {
			if err := utils.ExportOutputFile(appPath, appPath, bitriseAppPathEnvKey); err != nil {
				fail("Failed to export %s, error: %s", bitriseAppPathEnvKey, err)
			}

			log.Done("The app path is now available in the Environment Variable: %s (value: %s)", bitriseAppPathEnvKey, appPath)
		} else {
			if err := utils.ExportOutputFile(pkgPath, pkgPath, bitrisePKGPathEnvKey); err != nil {
				fail("Failed to export %s, error: %s", bitrisePKGPathEnvKey, err)
			}

			log.Done("The pkg path is now available in the Environment Variable: %s (value: %s)", bitrisePKGPathEnvKey, appPath)
		}
	} else {
		log.Info("Exporting with export options...")

		/*
		   Because of an RVM issue which conflicts with `xcodebuild`'s new
		   `-exportOptionsPlist` option
		   link: https://github.com/bitrise-io/steps-xcode-archive/issues/13
		*/
		if err := applyRVMFix(); err != nil {
			fail("rvm fix failed, error: %s", err)
		}

		if configs.CustomExportOptionsPlistContent != "" {
			log.Detail("custom export options content provided:")
			fmt.Println(configs.CustomExportOptionsPlistContent)

			if err := fileutil.WriteStringToFile(exportOptionsPath, configs.CustomExportOptionsPlistContent); err != nil {
				fail("Failed to write export options to file, error: %s", err)
			}
		} else {
			log.Detail("Generating export options")

			method, err := exportoptions.ParseMethod(configs.ExportMethod)
			if err != nil {
				fail("Failed to parse export options, error: %s", err)
			}

			var exportOpts exportoptions.ExportOptions
			if method == exportoptions.MethodAppStore {
				options := exportoptions.NewAppStoreOptions()
				options.UploadBitcode = (configs.UploadBitcode == "yes")
				options.TeamID = configs.TeamID

				exportOpts = options
			} else {
				options := exportoptions.NewNonAppStoreOptions(method)
				options.CompileBitcode = (configs.CompileBitcode == "yes")
				options.TeamID = configs.TeamID

				exportOpts = options
			}

			log.Detail("generated export options content:")
			fmt.Println()
			fmt.Println(exportOpts.String())

			if err := exportOpts.WriteToFile(exportOptionsPath); err != nil {
				fail("Failed to write export options to file, error: %s", err)
			}
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

		log.Done("$ %s", exportCmd.PrintableCmd())
		fmt.Println()

		if xcodebuildOut, err := exportCmd.RunAndReturnOutput(); err != nil {
			// xcdistributionlogs
			if logsDirPth, err := findIDEDistrubutionLogsPath(xcodebuildOut); err != nil {
				log.Warn("Failed to find xcdistributionlogs, error: %s", err)
			} else if err := utils.ExportOutputDirAsZip(logsDirPth, ideDistributionLogsZipPath, bitriseIDEDistributionLogsPthEnvKey); err != nil {
				log.Warn("Failed to export %s, error: %s", bitriseIDEDistributionLogsPthEnvKey, err)
			} else {
				log.Warn(`If you can't find the reason of the error in the log, please check the xcdistributionlogs
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
			if err := utils.ExportOutputFile(apps[0], appPath, bitriseAppPathEnvKey); err != nil {
				fail("Failed to export %s, error: %s", bitriseAppPathEnvKey, err)
			}

			log.Done("The app path is now available in the Environment Variable: %s (value: %s)", bitriseAppPathEnvKey, appPath)
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

			log.Done("The pkg path is now available in the Environment Variable: %s (value: %s)", bitrisePKGPathEnvKey, appPath)
		} else {
			fail("No app nor pkg output generated")
		}
	}
}
