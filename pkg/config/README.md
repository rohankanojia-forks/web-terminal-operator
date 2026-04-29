# Dynamic Image Mapping

This package provides dynamic image selection for Web Terminal based on the OpenShift cluster version.

## Overview

The Web Terminal Operator can now automatically select appropriate container images based on the detected OpenShift version. This allows different OpenShift versions to use version-specific tooling and exec images.

## Deployment with OLM

The `web-terminal-images` ConfigMap should be included in the operator bundle alongside the CSV. OLM will create the ConfigMap before starting the operator pod, preventing race conditions.

**Bundle Structure:**
```
bundle/
├── manifests/
│   ├── web-terminal.clusterserviceversion.yaml
│   └── web-terminal-images-configmap.yaml
└── metadata/
    └── annotations.yaml
```

When OLM installs the operator, it creates all resources in the `manifests/` directory before starting the operator deployment.

## How It Works

1. **Version Detection**: The operator queries the `ClusterVersion` resource to determine the OpenShift version
2. **ConfigMap Lookup**: Reads the `web-terminal-images` ConfigMap to find version-specific image mappings
3. **Image Selection**: Matches the detected version to configured images
4. **Fallback**: If version detection fails or no mapping exists, falls back to default images from environment variables

## Configuration

### ConfigMap Structure

The image mappings are defined in a ConfigMap located at `manifests/web-terminal-images-configmap.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: web-terminal-images
  namespace: openshift-operators
data:
  images.yaml: |
    mappings:
      - openshiftVersion: "4.14"
        toolingImage: "registry.redhat.io/web-terminal/web-terminal-tooling:4.14"
        execImage: "registry.redhat.io/web-terminal/web-terminal-exec:4.14"
    default:
      toolingImage: "${RELATED_IMAGE_web_terminal_tooling}"
      execImage: "${RELATED_IMAGE_web_terminal_exec}"
```

### Version Format

OpenShift versions are specified in `major.minor` format (e.g., `"4.14"`, `"4.15"`).

### Environment Variable Expansion

The ConfigMap supports environment variable expansion using `${VAR_NAME}` syntax in image strings. This is particularly useful for the default images.

## Fallback Behavior

The system gracefully falls back to defaults in these scenarios:

1. **ConfigMap Not Found**: Uses default images from environment variables (`RELATED_IMAGE_*`)
2. **No Matching Version**: Uses `default` section from ConfigMap, or environment variables if not specified
3. **ConfigMap Read Fails**: Uses default images from environment variables

## Adding New Versions or Updating Images

### For New Operator Releases (via OLM)

To add support for a new OpenShift version or update images:

1. Edit `manifests/web-terminal-images-configmap.yaml`
2. Add/update mapping entries with the version and images
3. Increment the operator version in the CSV
4. Build and publish the new operator bundle
5. OLM will update the ConfigMap when the operator is upgraded

**Example:** Publishing a patch release to update images for CVE fix:
```yaml
# Update the ConfigMap in the bundle
- openshiftVersion: "4.14"
  toolingImage: "registry.redhat.io/web-terminal/web-terminal-tooling:4.14.2"  # Was 4.14.1
  execImage: "registry.redhat.io/web-terminal/web-terminal-exec:4.14.2"
```

### For Manual Updates (Non-OLM or Testing)

For testing or non-OLM deployments:

1. Edit `manifests/web-terminal-images-configmap.yaml`
2. Add a new mapping entry with the version and images
3. Apply the ConfigMap: `kubectl apply -f manifests/web-terminal-images-configmap.yaml`
4. Restart the operator pod to pick up the new configuration

The operator will automatically use the new configuration when creating or updating templates.

## RBAC Requirements

The operator requires these additional permissions:

- `get` on `config.openshift.io/clusterversions` - to detect OpenShift version
- `get` on `configmaps` in the operator's namespace - to read image mappings

## Benefits

- **Version-Specific Tooling**: Different OpenShift versions can have different tool versions
- **CVE Management**: Easy to update images for specific OpenShift versions
- **No Operator Rebuild**: Image mappings can be updated without redeploying the operator
- **Backward Compatible**: Falls back to existing environment variable mechanism
