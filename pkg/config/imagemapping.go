//
// Copyright (c) 2021-2024 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	ImageConfigMapName      = "web-terminal-images"
	ImageConfigMapNamespace = "openshift-operators"
	ImageConfigMapKey       = "images.yaml"
)

// ImageMapping represents a single version-to-images mapping
type ImageMapping struct {
	OpenshiftVersion string `yaml:"openshiftVersion" json:"openshiftVersion"`
	ToolingImage     string `yaml:"toolingImage" json:"toolingImage"`
	ExecImage        string `yaml:"execImage" json:"execImage"`
}

// ImageConfig represents the full image configuration from ConfigMap
type ImageConfig struct {
	Mappings []ImageMapping `yaml:"mappings" json:"mappings"`
	Default  struct {
		ToolingImage string `yaml:"toolingImage" json:"toolingImage"`
		ExecImage    string `yaml:"execImage" json:"execImage"`
	} `yaml:"default" json:"default"`
}

// GetOpenShiftVersion detects the OpenShift version from the cluster
func GetOpenShiftVersion(ctx context.Context, client crclient.Client) (string, error) {
	clusterVersion := &configv1.ClusterVersion{}
	err := client.Get(ctx, types.NamespacedName{Name: "version"}, clusterVersion)
	if err != nil {
		return "", fmt.Errorf("failed to get ClusterVersion: %w", err)
	}

	// Get the current version from status history
	for _, update := range clusterVersion.Status.History {
		if update.State == configv1.CompletedUpdate {
			version := update.Version
			// Extract major.minor (e.g., "4.14" from "4.14.1")
			parts := strings.Split(version, ".")
			if len(parts) >= 2 {
				return fmt.Sprintf("%s.%s", parts[0], parts[1]), nil
			}
			return version, nil
		}
	}

	return "", fmt.Errorf("could not determine OpenShift version from ClusterVersion")
}

// loadImageConfig loads and parses the image configuration from ConfigMap
func loadImageConfig(ctx context.Context, client crclient.Client) (*ImageConfig, error) {
	namespace, err := GetNamespace()
	if err != nil {
		// Fall back to default namespace if we can't determine current namespace
		namespace = ImageConfigMapNamespace
	}

	configMap := &corev1.ConfigMap{}
	err = client.Get(ctx, types.NamespacedName{
		Name:      ImageConfigMapName,
		Namespace: namespace,
	}, configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap %s/%s: %w", namespace, ImageConfigMapName, err)
	}

	configData, ok := configMap.Data[ImageConfigMapKey]
	if !ok {
		return nil, fmt.Errorf("ConfigMap %s/%s does not contain key %s", namespace, ImageConfigMapName, ImageConfigMapKey)
	}

	var config ImageConfig
	if err := yaml.Unmarshal([]byte(configData), &config); err != nil {
		return nil, fmt.Errorf("failed to parse image configuration: %w", err)
	}

	return &config, nil
}

// expandEnvVars replaces ${VAR} patterns in image strings with environment variable values
func expandEnvVars(image string) string {
	// Simple expansion of ${VAR} patterns
	result := image
	start := strings.Index(result, "${")
	for start >= 0 {
		end := strings.Index(result[start:], "}")
		if end < 0 {
			break
		}
		end += start
		varName := result[start+2 : end]
		varValue := os.Getenv(varName)
		result = result[:start] + varValue + result[end+1:]
		start = strings.Index(result, "${")
	}
	return result
}

// GetImageForVersion returns the appropriate image based on OpenShift version
func GetImageForVersion(ctx context.Context, client crclient.Client, imageType string) (string, error) {
	// Try to get version-specific image
	version, err := GetOpenShiftVersion(ctx, client)
	if err != nil {
		// Not on OpenShift or can't detect version - use default
		return getDefaultImageForType(imageType)
	}

	// Load image configuration
	config, err := loadImageConfig(ctx, client)
	if err != nil {
		// Can't load config - use default
		return getDefaultImageForType(imageType)
	}

	// Find matching version in mappings
	for _, mapping := range config.Mappings {
		if mapping.OpenshiftVersion == version {
			var image string
			if imageType == "tooling" {
				image = mapping.ToolingImage
			} else {
				image = mapping.ExecImage
			}
			// Expand any environment variables in the image string
			return expandEnvVars(image), nil
		}
	}

	// No specific mapping found - try default from ConfigMap
	var defaultImage string
	if imageType == "tooling" {
		defaultImage = config.Default.ToolingImage
	} else {
		defaultImage = config.Default.ExecImage
	}

	if defaultImage != "" {
		// Expand environment variables in default image
		return expandEnvVars(defaultImage), nil
	}

	// Fall back to env var defaults
	return getDefaultImageForType(imageType)
}

// getDefaultImageForType returns the default image from environment variables
func getDefaultImageForType(imageType string) (string, error) {
	if imageType == "tooling" {
		return GetDefaultToolingImage()
	}
	return GetDefaultExecImage()
}
