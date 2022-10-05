package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/onsi/ginkgo/v2"
)

// HandleTestCaseFailure is responsible for attempting to collect relevant
// testing artifacts when individual test cases fail. In the case that
// a test passes, then this function is a no-op and will return a nil error.
func HandleTestCaseFailure() error {
	currentTest := ginkgo.CurrentSpecReport()
	if !currentTest.Failed() {
		return nil
	}

	// current test case failed. attempt to collect CI artifacts if the
	// $ARTIFACT_DIR environment variable has been set. This variable is
	// always present in downstream CI environments.
	artifactDir := os.Getenv("ARTIFACT_DIR")
	if artifactDir == "" {
		ginkgo.GinkgoT().Logf("not gathering testing artifacts as $ARTIFACT_DIR is unset")
		return nil
	}

	// create a dedicated test case directory to avoid overwriting the
	// testing artifacts gathered by a previous test case failure.
	testCaseDir := filepath.Join(artifactDir, strings.ReplaceAll(currentTest.LeafNodeText, " ", "-"))
	if err := os.MkdirAll(testCaseDir, os.ModePerm); err != nil {
		return err
	}

	cmd := exec.Command("/bin/bash", "-c", "./collect-ci-artifacts.sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	envVars := []string{
		"ARTIFACT_DIR=" + testCaseDir,
		"KUBECONFIG=" + os.Getenv("KUBECONFIG"),
		"KUBECTL=" + os.Getenv("KUBECTL"),
		"OPENSHIFT_CI=" + os.Getenv("OPENSHIFT_CI"),
	}
	cmd.Env = append(os.Environ(), envVars...)

	return cmd.Run()
}
