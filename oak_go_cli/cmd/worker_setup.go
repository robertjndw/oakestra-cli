package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// checkArch returns nil if the host architecture is amd64 or arm64.
func checkArch() error {
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return nil
	default:
		return fmt.Errorf("unsupported architecture %q (supported: amd64, arm64)", runtime.GOARCH)
	}
}

// checkSystemd returns nil if systemctl is reachable on PATH.
func checkSystemd() error {
	if _, err := exec.LookPath("systemctl"); err == nil {
		return nil
	}
	return fmt.Errorf("systemd not detected — NodeEngine requires a systemd-based Linux host")
}

// checkWorkerPrereqs validates that the host architecture and init system are
// compatible with NodeEngine. Called only on Linux.
func checkWorkerPrereqs() error {
	type check struct {
		name string
		fn   func() error
	}
	checks := []check{
		{"architecture (amd64/arm64)", checkArch},
		{"systemd", checkSystemd},
	}

	fmt.Println(bold("Checking worker prerequisites…"))
	var missing []string
	for _, c := range checks {
		if err := c.fn(); err != nil {
			fmt.Printf("  %s %s — %v\n", red("✗"), c.name, err)
			missing = append(missing, c.name)
		} else {
			fmt.Printf("  %s %s\n", green("✓"), c.name)
		}
	}
	fmt.Println()
	if len(missing) > 0 {
		return fmt.Errorf("worker prerequisites not met: %s", strings.Join(missing, ", "))
	}
	return nil
}

// configureGPU installs the NVIDIA Container Toolkit and nvidia-docker2,
// ported from the configure-gpu target in oakestra/oakestra PR #377.
// Requires apt and a systemd-based Linux host.
func configureGPU() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("GPU configuration is only supported on Linux")
	}
	if _, err := exec.LookPath("apt-get"); err != nil {
		return fmt.Errorf("apt-get not found — GPU configuration requires a Debian/Ubuntu host")
	}
	script := `set -eu
. /etc/os-release
: "${ID:?/etc/os-release does not set ID}"
: "${VERSION_ID:?/etc/os-release does not set VERSION_ID}"
DISTRIBUTION="${ID}${VERSION_ID}"
sudo apt-get install -y curl
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey \
  | sudo gpg --batch --yes --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
curl --fail -sL "https://nvidia.github.io/libnvidia-container/${DISTRIBUTION}/libnvidia-container.list" \
  | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' \
  | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
sudo apt-get update
sudo apt-get install -y libnvidia-container1 libnvidia-container-tools nvidia-docker2
`
	return runInteractive("sh", "-c", script)
}
