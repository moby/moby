package environment

// DanglingImageIdGraphDriver is the digest for dangling images used
// in tests when the graph driver is used. The graph driver image store
// identifies images by the ID of their config.
const DanglingImageIdGraphDriver = "sha256:0df1207206e5288f4a989a2f13d1f5b3c4e70467702c1d5d21dfc9f002b7bd43" // #nosec G101 -- ignoring: Potential hardcoded credentials (gosec)

// DanglingImageIdSnapshotter is the digest for dangling images used in
// tests when the containerd image store is used. The container image
// store identifies images by the ID of their manifest/manifest list..
const DanglingImageIdSnapshotter = "sha256:16d365089e5c10e1673ee82ab5bba38ade9b763296ad918bd24b42a1156c5456" // #nosec G101 -- ignoring: Potential hardcoded credentials (gosec)

func GetTestDanglingImageId(testEnv *Execution) string {
	if testEnv.UsingSnapshotter() {
		return DanglingImageIdSnapshotter
	}
	return DanglingImageIdGraphDriver
}
