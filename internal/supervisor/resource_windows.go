//go:build windows

package supervisor

func readProcessResources(pid int) (cpu float64, memMB float64) {
	return 0, 0
}
