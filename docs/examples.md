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