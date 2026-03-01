package ncnn

const supportedPlatformDescription = "linux/amd64, linux/arm64, darwin/amd64, darwin/arm64"

func isSupportedPlatform(goos, goarch string) bool {
	if goarch != "amd64" && goarch != "arm64" {
		return false
	}
	return goos == "linux" || goos == "darwin"
}
