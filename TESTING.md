# Testing Dynamic Image Selection

## Prerequisites

- Access to an OpenShift cluster (4.12+)
- Cluster admin permissions
- `oc` CLI tool installed and logged in
- `podman` or `docker` installed
- `skopeo` installed (for bundle deployment)

## Test Environment Setup

### 1. Build the Operator

#### Option A: Build Binary Only (for local testing)

```bash
# Build the operator binary
make compile

# Binary will be in: _output/bin/web-terminal-controller
```

#### Option B: Build Container Image (for cluster deployment)

```bash
# Set your image repository
export WTO_IMG=quay.io/YOUR_USERNAME/web-terminal-operator:test

# Build and push the controller image
make build_controller_image
```

### 2. Deploy to Cluster via OLM (Recommended)

This is the full deployment path using OLM, which handles the ConfigMap creation automatically.

```bash
# Set your image repositories
export WTO_IMG=quay.io/YOUR_USERNAME/web-terminal-operator:test
export BUNDLE_IMG=quay.io/YOUR_USERNAME/web-terminal-operator-metadata:test
export INDEX_IMG=quay.io/YOUR_USERNAME/web-terminal-operator-index:test

# Build, bundle, and install (all-in-one)
make build_install

# This command will:
# 1. Update CSV with your WTO_IMG
# 2. Build the operator bundle image
# 3. Build the index image
# 4. Register the catalog source
# 5. Create the operator subscription
# 6. Restore CSV to original state
```

**What happens:**
- OLM creates `web-terminal-images` ConfigMap (from bundle)
- OLM deploys the operator pod
- Operator reads ConfigMap and creates templates

#### Verify Deployment

```bash
# Check catalog source
oc get catalogsource -n openshift-marketplace | grep web-terminal

# Check subscription
oc get subscription web-terminal -n openshift-operators

# Check operator pod
oc get pods -n openshift-operators | grep web-terminal

# Check ConfigMap was created
oc get configmap web-terminal-images -n openshift-operators

# Check templates
oc get devworkspacetemplates -n openshift-operators
```

### 3. Manual ConfigMap Deployment (for local binary testing)

If you're running the operator binary locally (not via OLM):

```bash
# Deploy the ConfigMap manually
oc apply -f manifests/web-terminal-images-configmap.yaml
```

### 4. Running Locally (Alternative to OLM deployment)

For development/testing without OLM:

```bash
# Build binary
make compile

# Deploy ConfigMap first
oc apply -f manifests/web-terminal-images-configmap.yaml

# Set required environment variables
export RELATED_IMAGE_web_terminal_tooling="quay.io/wto/web-terminal-tooling:latest"
export RELATED_IMAGE_web_terminal_exec="quay.io/eclipse/che-machine-exec:nightly"
export WATCH_NAMESPACE="openshift-operators"  # Important for local development!

# Run the operator
./_output/bin/web-terminal-controller
```

**Note:** The `WATCH_NAMESPACE` env var is required when running locally because the operator can't read `/var/run/secrets/kubernetes.io/serviceaccount/namespace` outside of a pod.

### 5. Check OpenShift Version

```bash
# Verify what version the operator will detect
oc get clusterversion version -o jsonpath='{.status.history[0].version}'

# Example output: 4.14.8
# This means operator will look for "4.14" in ConfigMap
```

### 4. Verify ConfigMap

```bash
# Check ConfigMap exists
oc get configmap web-terminal-images -n openshift-operators

# View the mappings
oc get configmap web-terminal-images -n openshift-operators -o yaml
```

## Test Scenarios

### Test 1: Version-Specific Image Selection

**Setup:**
```bash
# Ensure your cluster version is in the ConfigMap
# For example, if running OCP 4.14, verify:
oc get configmap web-terminal-images -n openshift-operators -o yaml | grep "4.14"
```

**Expected behavior:**
- Operator should use 4.14-specific images from ConfigMap

**Run the operator:**
```bash
# Set required env vars (for fallback)
export RELATED_IMAGE_web_terminal_tooling="quay.io/wto/web-terminal-tooling:latest"
export RELATED_IMAGE_web_terminal_exec="quay.io/eclipse/che-machine-exec:nightly"
export WATCH_NAMESPACE="openshift-operators"

# Run operator (requires kubeconfig)
./_output/bin/web-terminal-controller
```

**Verify:**
```bash
# Check that templates were created
oc get devworkspacetemplates -n openshift-operators

# Check tooling template image
oc get devworkspacetemplate web-terminal-tooling -n openshift-operators -o jsonpath='{.spec.components[0].container.image}'

# Expected: registry.redhat.io/web-terminal/web-terminal-tooling:4.14

# Check exec template image
oc get devworkspacetemplate web-terminal-exec -n openshift-operators -o jsonpath='{.spec.components[0].container.image}'

# Expected: registry.redhat.io/web-terminal/web-terminal-exec:4.14
```

### Test 2: Fallback to Default Section

**Setup:**
```bash
# Temporarily edit ConfigMap to remove your version
oc edit configmap web-terminal-images -n openshift-operators
# Delete the mapping for your OCP version (e.g., remove 4.14 entry)
```

**Expected behavior:**
- Operator should use images from `default` section
- Which expands `${RELATED_IMAGE_*}` env vars

**Run and verify:**
```bash
# Delete existing templates
oc delete devworkspacetemplate web-terminal-tooling web-terminal-exec -n openshift-operators

# Run operator
./bin/web-terminal-operator

# Check images - should match env vars
oc get devworkspacetemplate web-terminal-tooling -n openshift-operators -o jsonpath='{.spec.components[0].container.image}'
# Expected: quay.io/wto/web-terminal-tooling:latest (from env var)
```

### Test 3: Fallback to Env Vars (No ConfigMap)

**Setup:**
```bash
# Delete ConfigMap
oc delete configmap web-terminal-images -n openshift-operators
```

**Expected behavior:**
- Operator logs warning about ConfigMap not found
- Falls back to env vars

**Run and verify:**
```bash
# Delete existing templates
oc delete devworkspacetemplate web-terminal-tooling web-terminal-exec -n openshift-operators

# Run operator (check logs)
./bin/web-terminal-operator 2>&1 | grep -i "configmap\|image"

# Should see fallback messages in logs

# Verify templates use env var images
oc get devworkspacetemplate web-terminal-tooling -n openshift-operators -o jsonpath='{.spec.components[0].container.image}'
# Expected: quay.io/wto/web-terminal-tooling:latest
```

### Test 4: Update Existing Templates

**Setup:**
```bash
# Create templates with old images first
export RELATED_IMAGE_web_terminal_tooling="quay.io/wto/web-terminal-tooling:old"
export RELATED_IMAGE_web_terminal_exec="quay.io/eclipse/che-machine-exec:old"

# Run operator to create templates
./bin/web-terminal-operator

# Verify old images
oc get devworkspacetemplate web-terminal-tooling -n openshift-operators -o jsonpath='{.spec.components[0].container.image}'
# Expected: quay.io/wto/web-terminal-tooling:old
```

**Test:**
```bash
# Re-apply ConfigMap with version mappings
oc apply -f manifests/web-terminal-images-configmap.yaml

# Run operator again
./bin/web-terminal-operator

# Verify templates were UPDATED (not recreated)
oc get devworkspacetemplate web-terminal-tooling -n openshift-operators -o jsonpath='{.spec.components[0].container.image}'
# Expected: registry.redhat.io/web-terminal/web-terminal-tooling:4.14

# Check template age (should be recent but not brand new if it existed)
oc get devworkspacetemplate web-terminal-tooling -n openshift-operators -o yaml | grep creationTimestamp
```

### Test 5: Environment Variable Expansion

**Setup:**
```bash
# Edit ConfigMap to use env vars in a mapping
oc edit configmap web-terminal-images -n openshift-operators
```

Add this mapping:
```yaml
- openshiftVersion: "4.14"
  toolingImage: "${RELATED_IMAGE_web_terminal_tooling}"  # Use env var
  execImage: "registry.redhat.io/web-terminal/web-terminal-exec:4.14"
```

**Test:**
```bash
# Set env var to specific image
export RELATED_IMAGE_web_terminal_tooling="quay.io/myrepo/custom-tooling:test"

# Delete and recreate templates
oc delete devworkspacetemplate web-terminal-tooling -n openshift-operators
./bin/web-terminal-operator

# Verify env var was expanded
oc get devworkspacetemplate web-terminal-tooling -n openshift-operators -o jsonpath='{.spec.components[0].container.image}'
# Expected: quay.io/myrepo/custom-tooling:test
```

## Unit Test (Code Verification)

You can also add a simple unit test:

```bash
# Create a test file
cat > pkg/config/imagemapping_test.go <<'EOF'
package config

import (
	"testing"
)

func TestExpandEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envKey   string
		envValue string
		expected string
	}{
		{
			name:     "simple expansion",
			input:    "${TEST_VAR}",
			envKey:   "TEST_VAR",
			envValue: "hello",
			expected: "hello",
		},
		{
			name:     "expansion in string",
			input:    "registry.io/${IMAGE_NAME}:tag",
			envKey:   "IMAGE_NAME",
			envValue: "myimage",
			expected: "registry.io/myimage:tag",
		},
		{
			name:     "no expansion needed",
			input:    "registry.io/image:tag",
			envKey:   "TEST_VAR",
			envValue: "value",
			expected: "registry.io/image:tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envValue)
			result := expandEnvVars(tt.input)
			if result != tt.expected {
				t.Errorf("expandEnvVars() = %v, want %v", result, tt.expected)
			}
		})
	}
}
EOF

# Run the test
go test ./pkg/config/...
```

## End-to-End Test

### Deploy Full Operator via OLM

```bash
# Build and push operator image
podman build -t quay.io/YOUR_USERNAME/web-terminal-operator:test .
podman push quay.io/YOUR_USERNAME/web-terminal-operator:test

# Update CSV with your image
sed -i 's|quay.io/wto/web-terminal-operator:next|quay.io/YOUR_USERNAME/web-terminal-operator:test|g' manifests/web-terminal.clusterserviceversion.yaml

# Create bundle
operator-sdk bundle create quay.io/YOUR_USERNAME/web-terminal-bundle:test
podman push quay.io/YOUR_USERNAME/web-terminal-bundle:test

# Install via OLM
operator-sdk run bundle quay.io/YOUR_USERNAME/web-terminal-bundle:test

# Verify ConfigMap was created by OLM
oc get configmap web-terminal-images -n openshift-operators

# Verify operator pod is running
oc get pods -n openshift-operators | grep web-terminal

# Check templates
oc get devworkspacetemplates -n openshift-operators
```

## Debugging

### Enable Verbose Logging

Edit main.go temporarily:
```go
ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
```

### Check Operator Logs

```bash
# If running locally
./bin/web-terminal-operator 2>&1 | tee operator.log

# If running in cluster
oc logs -f deployment/web-terminal-controller -n openshift-operators
```

### Verify RBAC Permissions

```bash
# Check if operator can read ClusterVersion
oc auth can-i get clusterversions.config.openshift.io --as=system:serviceaccount:openshift-operators:web-terminal-controller

# Check if operator can read ConfigMaps
oc auth can-i get configmaps --as=system:serviceaccount:openshift-operators:web-terminal-controller -n openshift-operators
```

## Expected Log Messages

**Success case:**
```
INFO	Syncing DevWorkspaceTemplate for Web Terminal Tooling
INFO	Web Terminal Tooling template updated.
INFO	Syncing DevWorkspaceTemplate for Web Terminal Exec
INFO	Web Terminal Exec template updated.
INFO	Web Terminal DevWorkspaceTemplates successfully set up.
```

**Fallback case (no ConfigMap):**
```
INFO	Syncing DevWorkspaceTemplate for Web Terminal Tooling
INFO	Could not load image mappings, using default images
INFO	DevWorkspaceTemplate for Web Terminal Tooling does not exist; creating.
```

## Cleanup

### If Deployed via OLM

```bash
# Uninstall the operator (removes everything)
make uninstall

# Unregister the catalog source
make unregister_catalogsource
```

### If Running Locally

```bash
# Stop the operator process (Ctrl+C)

# Delete templates
oc delete devworkspacetemplates web-terminal-tooling web-terminal-exec -n openshift-operators

# Delete ConfigMap
oc delete configmap web-terminal-images -n openshift-operators
```

## Quick Reference: Make Targets

```bash
make help                      # Show all available targets
make compile                   # Build operator binary
make build_controller_image    # Build and push container image
make build                     # Build bundle and index images
make build_install             # Build everything and install via OLM
make install                   # Install operator from existing bundle
make uninstall                 # Uninstall operator completely
make register_catalogsource    # Register catalog source
make unregister_catalogsource  # Unregister catalog source
make fmt                       # Format Go code
```
