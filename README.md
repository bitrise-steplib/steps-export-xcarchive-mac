# Export macOS Xcode archive

[![Step changelog](https://shields.io/github/v/release/bitrise-steplib/steps-export-xcarchive-mac?include_prereleases&label=changelog&color=blueviolet)](https://github.com/bitrise-steplib/steps-export-xcarchive-mac/releases)

Export macOS Xcode archive

<details>
<summary>Description</summary>

Export macOS Xcode archive.

Exports .app or .pkg from macOS .xcarchive.
</details>

## 🧩 Get started

Add this step directly to your workflow in the [Bitrise Workflow Editor](https://devcenter.bitrise.io/steps-and-workflows/steps-and-workflows-index/).

You can also run this step directly with [Bitrise CLI](https://github.com/bitrise-io/bitrise).

### Example

This step exports an app from an existing `xcarchive`. This is useful when one archive needs to be exported with different distribution methods:

```yaml
steps:
- certificate-and-profile-installer: {} # Requires certificates and profiles uploaded to Bitrise
- xcode-archive-mac:
    title: Archive and app store export
    inputs:
    - scheme: $BITRISE_SCHEME
    - export_method: app-store
- export-xcarchive-mac:
    title: Developer ID export
    inputs:
    - export_method: developer-id
    # Default input values of the step match the outputs of the previous step
- deploy-to-bitrise-io: {}
```

## ⚙️ Configuration

<details>
<summary>Inputs</summary>

| Key | Description | Flags | Default |
| --- | --- | --- | --- |
| `archive_path` | Path to the macOS archive (.xcarchive) which should be exported. |  | `$BITRISE_MACOS_XCARCHIVE_PATH` |
| `export_method` | Describes how Xcode should export the archive. | required | `development` |
| `upload_bitcode` | For __App Store__ exports, should the package include bitcode? | required | `yes` |
| `compile_bitcode` | For __non-App Store__ exports, should Xcode re-compile the app from bitcode? | required | `yes` |
| `team_id` | The Developer Portal team to use for this export.  Format example:  - `1MZX23ABCD4` |  |  |
| `custom_export_options_plist_content` | Specifies a custom export options plist content that configures archive exporting. If empty, step generates these options based on the embedded provisioning profile, with default values.  Auto generated export options available for export methods:  - app-store - ad-hoc - enterprise - development  If step doesn't find export method based on provisioning profile, development will be use.  Call `xcodebuild -help` for available export options. |  |  |
| `use_legacy_export` | If this input is set to `yes`, the step will use legacy export method. | required | `no` |
| `legacy_export_provisioning_profile_name` | If this input is empty, xcodebuild will grab one of the matching installed provisining profile. |  |  |
| `legacy_export_output_format` | Specify export format | required | `app` |
| `verbose_log` | Enable verbose logging? | required | `no` |
</details>

<details>
<summary>Outputs</summary>

| Environment Variable | Description |
| --- | --- |
| `BITRISE_APP_PATH` | The created macOS `.app` file's path |
| `BITRISE_PKG_PATH` | The created macOS `.pkg` file's path |
| `BITRISE_IDEDISTRIBUTION_LOGS_PATH` | Path to the `xcdistributionlogs` ZIP file |
</details>

## 🙋 Contributing

We welcome [pull requests](https://github.com/bitrise-steplib/steps-export-xcarchive-mac/pulls) and [issues](https://github.com/bitrise-steplib/steps-export-xcarchive-mac/issues) against this repository.

For pull requests, work on your changes in a forked repository and use the Bitrise CLI to [run step tests locally](https://devcenter.bitrise.io/bitrise-cli/run-your-first-build/).

**Note:** this step's end-to-end tests (defined in `e2e/bitrise.yml`) are working with secrets which are intentionally not stored in this repo. External contributors won't be able to run those tests. Don't worry, if you open a PR with your contribution, we will help with running tests and make sure that they pass.

Learn more about developing steps:

- [Create your own step](https://devcenter.bitrise.io/contributors/create-your-own-step/)
- [Testing your Step](https://devcenter.bitrise.io/contributors/testing-and-versioning-your-steps/)
