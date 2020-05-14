package v1alpha1

import (
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

var (
	GluonVersion    string
	GluonImage      string
	GluonPullPolicy corev1.PullPolicy
)

func init() {
	if v := os.Getenv("GLUON_VERSION"); v != "" {
		GluonVersion = v
	} else {
		GluonVersion = "latest"
	}

	if v := os.Getenv("GLUON_IMAGE"); v != "" {
		if strings.Contains(v, ":") {
			GluonImage = v
		} else {
			GluonImage = v + ":" + GluonVersion
		}
	} else {
		GluonImage = "starkandwayne/gluon-apparatus:" + GluonVersion
	}

	if v := os.Getenv("GLUON_PULL_POLICY"); v != "" {
		GluonPullPolicy = corev1.PullPolicy(v)
	} else {
		GluonPullPolicy = corev1.PullIfNotPresent
	}
}
