# Instructions to release a new version

To release a new version of the aws-sigv4-proxy, please follow these steps:

1. Create a release branch for this minor version series, if one does not exist yet. The convention is to name this branch: `release/v<release series>` where release series has the format `<major version>.<minor version>.x`. Example of branch `release/v1.8.x`
2. From the release branch, update the content of the `VERSION` file in the root of this repository. The convention is to ommit the patch version if that is in 0. Example of content: `1.8` or `1.8.1`. Merge the PR that updates the `VERSION` file. Confirm that the continuous integration workflow will succeed.
3. Run the release workflow. Go to the GitHub UI in this repository and select `Actions`. Then select the `Release aws-sigv4-proxy` workflow. Select the release branch. You can optionally test with dry-run mode before releasing.
4. After the release is completed. Update the release notes for this release.
5. Merge the changes from the release branch into mainline.
