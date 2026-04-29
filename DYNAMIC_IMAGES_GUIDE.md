# Dynamic Image Selection - Deployment Guide

## How It Works

The Web Terminal Operator now supports version-specific container images based on the detected OpenShift version. This guide explains how the system works and how to deploy/update it.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ OLM Operator Bundle                                         │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ manifests/                                              │ │
│ │   ├── web-terminal.clusterserviceversion.yaml          │ │
│ │   └── web-terminal-images-configmap.yaml               │ │
│ └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ OLM installs
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ OpenShift Cluster                                           │
│                                                             │
│  1. ConfigMap Created First                                │
│     (web-terminal-images in openshift-operators)           │
│                                                             │
│  2. Operator Pod Starts                                    │
│     └─► Reads ClusterVersion → "4.14"                      │
│     └─► Reads ConfigMap → finds "4.14" mapping             │
│     └─► Uses: tooling:4.14 and exec:4.14                   │
│                                                             │
│  3. Creates DevWorkspaceTemplates                          │
│     └─► web-terminal-tooling (with 4.14 image)             │
│     └─► web-terminal-exec (with 4.14 image)                │
└─────────────────────────────────────────────────────────────┘
```

## No Race Condition with OLM

When using OLM (Operator Lifecycle Manager), there is **no race condition** because:

1. **OLM creates ConfigMap BEFORE operator pod starts**
   - All resources in `manifests/` directory are created first
   - Only after all resources exist does OLM start the deployment

2. **Operator startup sequence:**
   ```
   Step 1: OLM creates web-terminal-images ConfigMap
   Step 2: OLM creates ServiceAccount, RBAC
   Step 3: OLM creates Deployment
   Step 4: Operator pod starts
   Step 5: Operator reads ConfigMap (already exists!)
   Step 6: Operator creates templates with correct images
   ```

## Deployment Methods

### Method 1: OLM Bundle (Recommended for Production)

**Bundle Structure:**
```
bundle/
├── manifests/
│   ├── web-terminal.clusterserviceversion.yaml
│   └── web-terminal-images-configmap.yaml
└── metadata/
    └── annotations.yaml
```

**Deployment:**
```bash
# Build bundle
operator-sdk bundle create quay.io/myorg/web-terminal-bundle:v1.16.0

# Push bundle
podman push quay.io/myorg/web-terminal-bundle:v1.16.0

# Install via OLM
operator-sdk run bundle quay.io/myorg/web-terminal-bundle:v1.16.0
```

### Method 2: Direct Installation (Testing/Development)

**For non-OLM environments**, manually ensure ConfigMap exists first:

```bash
# Step 1: Create ConfigMap FIRST
kubectl apply -f manifests/web-terminal-images-configmap.yaml

# Step 2: Deploy operator
kubectl apply -f deploy/operator.yaml
```

## Updating Images

### Scenario: CVE Fix Requires Image Update

You need to update the tooling image for OpenShift 4.14 to fix a CVE.

**Steps:**

1. **Update ConfigMap in bundle:**
   ```yaml
   # manifests/web-terminal-images-configmap.yaml
   mappings:
     - openshiftVersion: "4.14"
       toolingImage: "registry.redhat.io/web-terminal/web-terminal-tooling@sha256:new-digest"  # Updated
       execImage: "registry.redhat.io/web-terminal/web-terminal-exec:4.14"
   ```

2. **Update CSV version:**
   ```yaml
   # manifests/web-terminal.clusterserviceversion.yaml
   metadata:
     name: web-terminal.v1.16.1  # Was v1.16.0
   spec:
     version: 1.16.1  # Was 1.16.0
     replaces: web-terminal.v1.16.0
   ```

3. **Build and publish new bundle:**
   ```bash
   operator-sdk bundle create quay.io/myorg/web-terminal-bundle:v1.16.1
   podman push quay.io/myorg/web-terminal-bundle:v1.16.1
   ```

4. **OLM automatically upgrades:**
   - OLM detects new version
   - Updates ConfigMap with new image
   - Restarts operator pod
   - Operator syncs templates with new image

## Fallback Behavior

The system gracefully handles missing ConfigMap or version mismatches:

```
┌─────────────────────────────────────────────────────────┐
│ Operator Startup on OpenShift                           │
└─────────────────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│ Read ClusterVersion resource → "4.14"                   │
└─────────────────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│ Try to read ConfigMap                                   │
└─────────────────────────────────────────────────────────┘
     │ Success                    │ Fail/Not Found
     ▼                            ▼
┌─────────────────┐      ┌──────────────────────────┐
│ Find version    │      │ Use env var defaults     │
│ mapping "4.14"  │      │ RELATED_IMAGE_*          │
└─────────────────┘      └──────────────────────────┘
     │ Found     │ Not Found      │
     ▼           ▼                │
┌────────────┐ ┌─────────────┐   │
│ Use mapped │ │ Use default │   │
│ images     │ │ from CM     │   │
└────────────┘ └─────────────┘   │
     │               │            │
     └───────────────┴────────────┘
                     ▼
          ┌──────────────────────┐
          │ Create templates     │
          └──────────────────────┘
```

**Fallback scenarios:**
1. ConfigMap doesn't exist → uses `RELATED_IMAGE_*` env vars
2. Version not in mappings → uses `default` section from ConfigMap
3. ConfigMap `default` section empty → uses `RELATED_IMAGE_*` env vars

## Benefits

✅ **No operator rebuild** - Just update the bundle ConfigMap
✅ **Version-specific images** - Different OCP versions get appropriate tools
✅ **CVE management** - Quick updates via bundle patch releases
✅ **Backward compatible** - Falls back to existing env var mechanism
✅ **No race conditions** - OLM creates ConfigMap before operator starts

## Backward Compatibility

### Upgrading from Old Version (v1.15.0) → New Version (v1.16.0)

**What happens:**
1. OLM creates `web-terminal-images` ConfigMap
2. OLM updates operator deployment to v1.16.0
3. New operator pod starts
4. Operator reads ClusterVersion → gets "4.14" (example)
5. Operator reads ConfigMap → finds 4.14 mapping
6. Operator **updates existing templates** with new images
7. Users get version-specific images automatically

**Existing templates are updated, not recreated:**
- Preserves any metadata/labels
- Updates only the container image field
- No disruption to running web terminals

### If ConfigMap is Missing (e.g., manual deployment)

Old behavior is preserved:
- Operator tries to read ConfigMap → fails
- Falls back to `RELATED_IMAGE_*` env vars
- Creates templates exactly like v1.15.0 did

**Result:** ✅ Fully backward compatible, zero breaking changes

### Unmanaged Templates

Users who customized templates with annotations are respected:
```yaml
metadata:
  annotations:
    controller.devfile.io/unmanaged: "true"
```

**Behavior:** Template is still updated **only if** it uses a default image (for CVE fixes). Custom images are preserved.
