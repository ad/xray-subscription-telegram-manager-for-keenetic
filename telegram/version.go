package telegram

// SetVersionInfo sets the version information from main package
func SetVersionInfo(version, buildTime, goVersion string) {
	CurrentVersion = version
	BuildTime = buildTime
	GoVersion = goVersion
}
